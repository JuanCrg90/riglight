package logs

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestReadHelperSSERequiresModeSourceAndSequenceBindings(t *testing.T) {
	valid := "event: log-status\ndata: {\"schemaVersion\":1,\"mode\":\"LIVE\",\"sourceId\":\"app\",\"state\":\"OPENING\",\"reasonCode\":null}\n\n" +
		"event: log-status\ndata: {\"schemaVersion\":1,\"mode\":\"LIVE\",\"sourceId\":\"app\",\"state\":\"STREAMING\",\"reasonCode\":null}\n\n" +
		"event: log\ndata: {\"schemaVersion\":1,\"mode\":\"LIVE\",\"sourceId\":\"app\",\"seq\":1,\"text\":\"line\",\"stream\":\"STDOUT\",\"truncated\":false}\n\n" +
		"event: log-status\ndata: {\"schemaVersion\":1,\"mode\":\"LIVE\",\"sourceId\":\"app\",\"state\":\"ENDED\",\"reasonCode\":\"EOF\"}\n\n"
	var got []string
	if err := readHelperSSE(context.Background(), strings.NewReader(valid), "app", func(_ Stream, text string, _ bool) error { got = append(got, text); return nil }); err != nil || len(got) != 1 || got[0] != "line" {
		t.Fatalf("result = %#v, %v", got, err)
	}
	for _, bad := range []string{strings.Replace(valid, `"sourceId":"app"`, `"sourceId":"other"`, 1), strings.Replace(valid, `"seq":1`, `"seq":2`, 1), strings.Replace(valid, `"mode":"LIVE"`, `"mode":"DEMO"`, 1)} {
		if err := readHelperSSE(context.Background(), strings.NewReader(bad), "app", func(Stream, string, bool) error { return nil }); !errors.Is(err, ErrInvalid) {
			t.Errorf("error = %v", err)
		}
	}
}
