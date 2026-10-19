package containers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProjectedSourceAcceptsLiveNonAvailableOverviews(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`{"schemaVersion":1,"sampledAt":"2026-01-01T00:00:05Z","mode":"LIVE","status":"NOT_READY","reasonCode":null,"ageMs":null,"containers":[]}`,
		`{"schemaVersion":1,"sampledAt":"2026-01-01T00:00:05Z","mode":"LIVE","status":"UNAVAILABLE","reasonCode":"TIMEOUT","ageMs":null,"containers":[]}`,
		`{"schemaVersion":1,"sampledAt":"2026-01-01T00:00:05Z","mode":"LIVE","status":"UNSUPPORTED","reasonCode":"API_UNSUPPORTED","ageMs":null,"containers":[]}`,
	} {
		sample, err := decodeProjected([]byte(body), time.Now(), 0)
		if err != nil || sample.Status == OverviewAvailable {
			t.Fatalf("sample=%#v err=%v", sample, err)
		}
	}
}
func TestProjectedSourceRejectsOffsetTimestampsAndCombinedAgeOverflow(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		strings.Replace(projectedJSON(1), `2026-01-01T00:00:05Z`, `2026-01-01T01:00:05+01:00`, 1),
		projectedJSON(int64(^uint64(0)>>1) / int64(time.Millisecond)),
	} {
		if _, err := decodeProjected([]byte(body), time.Now(), time.Nanosecond); err == nil {
			t.Fatalf("accepted unsafe projection %s", body)
		}
	}
	valid := strings.Replace(projectedJSON(1), `00:00:05Z`, `00:00:05.123456789Z`, -1)
	if _, err := decodeProjected([]byte(valid), time.Now(), 0); err != nil {
		t.Fatalf("rejected UTC nanoseconds: %v", err)
	}
}
func TestHTTPSourceClassifiesSlowBodyAsTimeout(t *testing.T) {
	t.Parallel()
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer fixture.Close()
	source, err := NewHTTPSource(fixture.URL)
	if err != nil {
		t.Fatal(err)
	}
	sample, err := source.Collect(context.Background())
	if err != nil || sample.Status != OverviewUnavailable || sample.ReasonCode == nil || *sample.ReasonCode != ReasonTimeout {
		t.Fatalf("sample=%#v err=%v", sample, err)
	}
}
