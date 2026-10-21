package dockerproxy

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

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
func TestLogProviderBoundsQuietInspectBody(t *testing.T) {
	original := inspectTimeout
	inspectTimeout = 20 * time.Millisecond
	t.Cleanup(func() { inspectTimeout = original })
	canceled := make(chan struct{})
	socket, stop := fakeDaemon(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
		close(canceled)
	}))
	defer stop()
	config, _ := NewConfig(socket, "127.0.0.1:9123", []string{"app=service"})
	provider, _ := NewLogProvider(config)
	defer provider.CloseIdleConnections()
	started := time.Now()
	err := provider.Stream(context.Background(), "app", func() error { return nil }, func(logs.Stream, string, bool) error { return nil })
	if !errors.Is(err, logs.ErrInvalid) || time.Since(started) > time.Second {
		t.Fatalf("quiet inspect = %v after %s", err, time.Since(started))
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("quiet inspect body was not canceled")
	}
}

func TestLogProviderContentTypeMatchesTTYAndFraming(t *testing.T) {
	framed := dockerFrame(1, []byte("café\n"))
	plain := []byte("café\n")
	cases := []struct {
		name        string
		contentType string
		tty         bool
		body        []byte
		wantErr     bool
	}{
		{name: "raw stream multiplexed", contentType: "application/vnd.docker.raw-stream", body: framed},
		{name: "octet stream multiplexed", contentType: "application/octet-stream", body: framed},
		{name: "multiplexed stream", contentType: "application/vnd.docker.multiplexed-stream", body: framed},
		{name: "multiplexed stream with valid parameter", contentType: "application/vnd.docker.multiplexed-stream; charset=utf-8", body: framed},
		{name: "raw stream tty", contentType: "application/vnd.docker.raw-stream", tty: true, body: plain},
		{name: "octet stream tty", contentType: "application/octet-stream", tty: true, body: plain},
		{name: "multiplexed stream tty mismatch", contentType: "application/vnd.docker.multiplexed-stream", tty: true, body: framed, wantErr: true},
		{name: "invalid charset parameter", contentType: `application/vnd.docker.multiplexed-stream; charset="`, body: framed, wantErr: true},
		{name: "garbage content type", contentType: "not a media type", body: framed, wantErr: true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			socket, stop := fakeDaemon(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1.44/containers/service/json" {
					_, _ = io.WriteString(w, `{"Id":"`+testID+`","Name":"/service","Config":{"Tty":`+map[bool]string{true: "true", false: "false"}[test.tty]+`}}`)
					return
				}
				w.Header().Set("Content-Type", test.contentType)
				_, _ = w.Write(test.body)
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

			ready := false
			var lines []string
			err = provider.Stream(context.Background(), "app", func() error {
				ready = true
				return nil
			}, func(_ logs.Stream, line string, _ bool) error {
				lines = append(lines, line)
				return nil
			})
			if test.wantErr {
				if !errors.Is(err, logs.ErrInvalid) || ready || len(lines) != 0 {
					t.Fatalf("mismatch accepted: err=%v ready=%v lines=%q", err, ready, lines)
				}
				return
			}
			if err != nil || !ready || len(lines) != 1 || lines[0] != "café" {
				t.Fatalf("stream = err %v, ready %v, lines %q", err, ready, lines)
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
