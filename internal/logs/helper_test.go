package logs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func helperFrames(terminal bool) string {
	value := "event: log-status\ndata: {\"schemaVersion\":1,\"mode\":\"LIVE\",\"sourceId\":\"docker.app\",\"state\":\"OPENING\",\"reasonCode\":null}\n\n" +
		"event: log-status\ndata: {\"schemaVersion\":1,\"mode\":\"LIVE\",\"sourceId\":\"docker.app\",\"state\":\"STREAMING\",\"reasonCode\":null}\n\n" +
		"event: log\ndata: {\"schemaVersion\":1,\"mode\":\"LIVE\",\"sourceId\":\"docker.app\",\"seq\":1,\"text\":\"line\",\"stream\":\"STDOUT\",\"truncated\":false}\n\n"
	if terminal {
		value += "event: log-status\ndata: {\"schemaVersion\":1,\"mode\":\"LIVE\",\"sourceId\":\"docker.app\",\"state\":\"ENDED\",\"reasonCode\":\"EOF\"}\n\n"
	}
	return value
}

func TestReadHelperSSERequiresModeSourceAndSequenceBindings(t *testing.T) {
	valid := helperFrames(true)
	var got []string
	if err := readHelperSSE(context.Background(), strings.NewReader(valid), "docker.app", func(_ Stream, text string, _ bool) error { got = append(got, text); return nil }); err != nil || len(got) != 1 || got[0] != "line" {
		t.Fatalf("result = %#v, %v", got, err)
	}
	for _, bad := range []string{strings.Replace(valid, `"sourceId":"docker.app"`, `"sourceId":"docker.other"`, 1), strings.Replace(valid, `"seq":1`, `"seq":2`, 1), strings.Replace(valid, `"mode":"LIVE"`, `"mode":"DEMO"`, 1)} {
		if err := readHelperSSE(context.Background(), strings.NewReader(bad), "docker.app", func(Stream, string, bool) error { return nil }); !errors.Is(err, ErrInvalid) {
			t.Errorf("error = %v", err)
		}
	}
}

func TestHelperWireRejectsMissingDuplicateAndNullFieldsBeforeEmit(t *testing.T) {
	for _, raw := range []string{
		`{"schemaVersion":1,"mode":"LIVE","sourceId":"docker.app","seq":1,"stream":"STDOUT","truncated":false}`,
		`{"schemaVersion":1,"mode":"LIVE","sourceId":"docker.app","seq":1,"text":"one","text":"two","stream":"STDOUT","truncated":false}`,
		`{"schemaVersion":1,"mode":"LIVE","sourceId":"docker.app","seq":1,"text":null,"stream":"STDOUT","truncated":false}`,
		`{"schemaVersion":1,"mode":"LIVE","sourceId":"docker.app","seq":1,"text":"one","stream":"STDOUT","truncated":null}`,
	} {
		if _, ok := parseHelperEvent([]byte(raw)); ok {
			t.Errorf("event accepted: %s", raw)
		}
	}
	for _, raw := range []string{
		`{"schemaVersion":1,"aliases":[]}`,
		`{"schemaVersion":1,"enabled":false,"enabled":true,"aliases":[]}`,
		`{"schemaVersion":1,"enabled":null,"aliases":[]}`,
		`{"schemaVersion":1,"enabled":false,"aliases":null}`,
	} {
		_, _, ok := parseHelperCatalog([]byte(raw))
		if ok {
			t.Errorf("catalog accepted: %s", raw)
		}
	}
	bad := strings.Replace(helperFrames(true), `,"text":"line"`, "", 1)
	called := 0
	if err := readHelperSSE(context.Background(), strings.NewReader(bad), "docker.app", func(Stream, string, bool) error { called++; return nil }); !errors.Is(err, ErrInvalid) || called != 0 {
		t.Fatalf("invalid event result = %v, emitted = %d", err, called)
	}
}

func TestHelperSSERequiresTerminalFrameAndCompleteFraming(t *testing.T) {
	for _, body := range []string{"", helperFrames(false), "event: log-status"} {
		if err := readHelperSSE(context.Background(), strings.NewReader(body), "docker.app", func(Stream, string, bool) error { return nil }); !errors.Is(err, ErrInvalid) {
			t.Errorf("body %q error = %v", body, err)
		}
	}
	oversized := "event: " + strings.Repeat("x", MaxWireBytes/2) + "\ndata: " + strings.Repeat("y", MaxWireBytes/2) + "\n\n"
	if err := readHelperSSE(context.Background(), strings.NewReader(oversized), "docker.app", func(Stream, string, bool) error { return nil }); !errors.Is(err, ErrInvalid) {
		t.Fatalf("oversized frame = %v", err)
	}
}

func TestHelperCatalogReadHasAnAbsoluteDeadline(t *testing.T) {
	original := catalogTimeout
	catalogTimeout = 20 * time.Millisecond
	t.Cleanup(func() { catalogTimeout = original })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	helper, err := NewHelperDockerCatalog(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer helper.CloseIdleConnections()
	started := time.Now()
	_, err = helper.catalog(context.Background())
	if !errors.Is(err, ErrInvalid) || time.Since(started) > time.Second {
		t.Fatalf("catalog read = %v after %s", err, time.Since(started))
	}
}
