package dockerproxy

import (
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"testing"

	"riglight/internal/logs"
)

func TestLogProviderPinsInspectIdentityAndParsesSplitFrames(t *testing.T) {
	socket, stop := fakeDaemon(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.44/containers/service/json":
			if r.URL.RawQuery != "" || r.Method != http.MethodGet {
				t.Errorf("inspect = %s %s", r.Method, r.URL)
			}
			_, _ = io.WriteString(w, `{"Id":"`+testID+`","Name":"/service","Config":{"Tty":false,"Env":["INSPECT_SECRET"]}}`)
		case "/v1.44/containers/" + testID + "/logs":
			if r.URL.RawQuery != "stdout=1&stderr=1&follow=1&tail=200&timestamps=0" {
				t.Errorf("query = %q", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
			_, _ = w.Write(dockerFrame(1, []byte("out\n")))
			_, _ = w.Write(dockerFrame(2, []byte("err\n")))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer stop()
	config, err := NewConfig(socket, "127.0.0.1:9123", []string{"app=service"})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewLogProvider(config)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.CloseIdleConnections()
	var got []struct {
		stream logs.Stream
		text   string
	}
	err = provider.Stream(context.Background(), "app", func() error { return nil }, func(stream logs.Stream, text string, _ bool) error {
		got = append(got, struct {
			stream logs.Stream
			text   string
		}{stream, text})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].stream != logs.StreamStdout || got[0].text != "out" || got[1].stream != logs.StreamStderr || got[1].text != "err" {
		t.Fatalf("lines = %#v", got)
	}
}

func TestLogProviderRejectsMalformedFramesAndMissingTTY(t *testing.T) {
	for _, test := range []struct{ name, inspect, body string }{
		{"missing tty", `{"Id":"` + testID + `","Name":"/service","Config":{}}`, ""},
		{"malformed frame", `{"Id":"` + testID + `","Name":"/service","Config":{"Tty":false}}`, "\x03\x00\x00\x00\x00\x00\x00\x01x"},
	} {
		t.Run(test.name, func(t *testing.T) {
			socket, stop := fakeDaemon(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1.44/containers/service/json" {
					_, _ = io.WriteString(w, test.inspect)
					return
				}
				w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
				_, _ = io.WriteString(w, test.body)
			}))
			defer stop()
			config, _ := NewConfig(socket, "127.0.0.1:9123", []string{"app=service"})
			provider, _ := NewLogProvider(config)
			if err := provider.Stream(context.Background(), "app", func() error { return nil }, func(logs.Stream, string, bool) error { return nil }); err == nil {
				t.Fatal("malformed response accepted")
			}
		})
	}
}
func dockerFrame(stream byte, payload []byte) []byte {
	frame := make([]byte, 8+len(payload))
	frame[0] = stream
	binary.BigEndian.PutUint32(frame[4:], uint32(len(payload)))
	copy(frame[8:], payload)
	return frame
}
