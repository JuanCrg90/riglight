package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

const helperTestID = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestParseConfigRequiresExplicitSocketAndSelection(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"--socket", "/tmp/fake.sock"}, {"--container", "app=service"}, {"--socket", "relative", "--container", "app=service"}, {"--socket", "/tmp/fake.sock", "--container", "app=service", "--listen", "0.0.0.0:9123"}} {
		if _, err := parseConfig(args); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
	config, err := parseConfig([]string{"--socket", "/tmp/fake.sock", "--container", "app=service"})
	if err != nil || config.Listen != "127.0.0.1:9123" {
		t.Fatalf("config = %#v, %v", config, err)
	}
}

func TestRunCancellationClosesQuietLogHandler(t *testing.T) {
	socket := t.TempDir() + "/daemon.sock"
	daemonListener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	quietStarted, quietCanceled := make(chan struct{}), make(chan struct{})
	var startedOnce, canceledOnce sync.Once
	daemon := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.44/containers/service/json":
			_, _ = io.WriteString(w, `{"Id":"`+helperTestID+`","Name":"/service","State":{"Status":"running"},"Config":{"Tty":false}}`)
		case "/v1.44/containers/" + helperTestID + "/stats":
			_, _ = io.WriteString(w, `{"id":"`+helperTestID+`","cpu_stats":{"cpu_usage":{"total_usage":1,"percpu_usage":[1]},"system_cpu_usage":1,"online_cpus":1},"memory_stats":{"usage":1,"limit":1,"stats":{"inactive_file":0}}}`)
		case "/v1.44/containers/" + helperTestID + "/logs":
			w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
			w.WriteHeader(http.StatusOK)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			startedOnce.Do(func() { close(quietStarted) })
			<-r.Context().Done()
			canceledOnce.Do(func() { close(quietCanceled) })
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})}
	go func() { _ = daemon.Serve(daemonListener) }()
	defer func() { _ = daemon.Close(); _ = daemonListener.Close() }()
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := probe.Addr().String()
	_ = probe.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{"--socket", socket, "--container", "app=service", "--logs", "--listen", address})
	}()
	requestDeadline := time.Now().Add(time.Second)
	var response *http.Response
	for {
		request, requestErr := http.NewRequest(http.MethodGet, "http://"+address+"/api/v1/logs/app", nil)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		request.Host = address
		response, requestErr = (&http.Client{Transport: &http.Transport{Proxy: nil}}).Do(request)
		if requestErr == nil {
			break
		}
		if time.Now().After(requestDeadline) {
			t.Fatalf("helper did not start: %v", requestErr)
		}
		time.Sleep(time.Millisecond)
	}
	defer response.Body.Close()
	select {
	case <-quietStarted:
	case <-time.After(time.Second):
		t.Fatal("quiet log did not open")
	}
	cancel()
	select {
	case <-quietCanceled:
	case <-time.After(time.Second):
		t.Fatal("quiet log handler was not canceled")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("run did not stop within shared shutdown budget")
	}
}
