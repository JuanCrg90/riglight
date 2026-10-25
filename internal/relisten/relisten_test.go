package relisten

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunPreCanceledContextDoesNotStartServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	listener := &testListener{closed: make(chan struct{})}
	var starts atomic.Int32
	start := func(*http.Server, net.Listener, chan<- error) { starts.Add(1) }
	server := &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})}
	if err := run(ctx, server, listener, nil, nil, nil, start); !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("Run error = %v, want %v", err, http.ErrServerClosed)
	}
	if got := starts.Load(); got != 0 {
		t.Fatalf("server start calls = %d, want 0", got)
	}
	select {
	case <-listener.closed:
	default:
		t.Fatal("listener was not closed")
	}
}

func TestRunCancellationClosesListener(t *testing.T) {
	listener := &testListener{closed: make(chan struct{})}
	server := &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, server, listener, nil, nil, nil) }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("Run error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not stop")
	}
}

func TestRunRebindsAcrossReadinessChurn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initial := listen(t)
	listeners := []net.Listener{listen(t), listen(t)}
	all := append([]net.Listener{initial}, listeners...)
	var state atomic.Int32
	state.Store(1)
	var binds atomic.Int32
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "same handler") })}
	ready := func() (bool, error) {
		if state.Load() == 3 {
			return false, errors.New("lookup unavailable")
		}
		return state.Load() == 1, nil
	}
	bind := func(context.Context) (net.Listener, error) {
		i := int(binds.Add(1)) - 1
		if i >= len(listeners) {
			return nil, errors.New("unexpected bind")
		}
		return listeners[i], nil
	}
	done := make(chan error, 1)
	go func() { done <- Run(ctx, server, initial, ready, bind, nil) }()
	getBody(t, initial.Addr().String(), "same handler")
	state.Store(0)
	waitFor(t, func() bool { return closed(initial) })
	state.Store(1)
	waitFor(t, func() bool { return binds.Load() == 1 })
	getBody(t, listeners[0].Addr().String(), "same handler")
	state.Store(0)
	waitFor(t, func() bool { return closed(listeners[0]) })
	state.Store(3)
	if binds.Load() != 1 {
		t.Fatal("rebound while readiness lookup errored")
	}
	state.Store(1)
	waitFor(t, func() bool { return binds.Load() == 2 })
	getBody(t, listeners[1].Addr().String(), "same handler")
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("Run error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run leaked")
	}
	for _, listener := range all {
		if !closed(listener) {
			t.Errorf("listener %v remains open", listener.Addr())
		}
	}
	if binds.Load() != 2 {
		t.Fatalf("bind calls = %d", binds.Load())
	}
}

func TestRunCancelWhileWaitingDoesNotBind(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initial := listen(t)
	var state atomic.Int32
	state.Store(1)
	var binds atomic.Int32
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })}
	ready := func() (bool, error) { return state.Load() == 1, nil }
	bind := func(context.Context) (net.Listener, error) { binds.Add(1); return listen(t), nil }
	done := make(chan error, 1)
	go func() { done <- Run(ctx, server, initial, ready, bind, nil) }()
	getBody(t, initial.Addr().String(), "ok")
	state.Store(0)
	waitFor(t, func() bool { return closed(initial) })
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("Run error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run leaked")
	}
	if binds.Load() != 0 {
		t.Fatalf("bind calls = %d, want 0 after cancellation", binds.Load())
	}
}

func TestRunPersistentReadinessErrorWaitsFailClosed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initial := listen(t)
	var binds atomic.Int32
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })}
	ready := func() (bool, error) { return false, errors.New("lookup unavailable") }
	bind := func(context.Context) (net.Listener, error) { binds.Add(1); return listen(t), nil }
	done := make(chan error, 1)
	go func() { done <- Run(ctx, server, initial, ready, bind, nil) }()
	getBody(t, initial.Addr().String(), "ok")
	waitFor(t, func() bool { return closed(initial) })
	select {
	case err := <-done:
		t.Fatalf("Run returned early: %v", err)
	case <-time.After(5 * time.Second):
	}
	if binds.Load() != 0 {
		t.Fatalf("bind calls = %d, want 0 while readiness keeps erroring", binds.Load())
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("Run error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run leaked")
	}
}

func TestRunBindFailureAfterRestoreIsFatal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initial := listen(t)
	var state atomic.Int32
	state.Store(1)
	bindErr := errors.New("bind failed")
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })}
	ready := func() (bool, error) { return state.Load() == 1, nil }
	bind := func(context.Context) (net.Listener, error) { return nil, bindErr }
	done := make(chan error, 1)
	go func() { done <- Run(ctx, server, initial, ready, bind, nil) }()
	getBody(t, initial.Addr().String(), "ok")
	state.Store(0)
	waitFor(t, func() bool { return closed(initial) })
	state.Store(1)
	select {
	case err := <-done:
		if !errors.Is(err, bindErr) {
			t.Fatalf("Run error = %v, want bind failure", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not stop after bind failure")
	}
}

func TestRunCancelDuringBindDoesNotStartNewServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initial := listen(t)
	var state atomic.Int32
	state.Store(1)
	var binds atomic.Int32
	var rebound net.Listener
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })}
	ready := func() (bool, error) { return state.Load() == 1, nil }
	bind := func(ctx context.Context) (net.Listener, error) {
		binds.Add(1)
		<-ctx.Done()
		rebound = listen(t)
		return rebound, nil
	}
	done := make(chan error, 1)
	go func() { done <- Run(ctx, server, initial, ready, bind, nil) }()
	getBody(t, initial.Addr().String(), "ok")
	state.Store(0)
	waitFor(t, func() bool { return closed(initial) })
	state.Store(1)
	deadline := time.Now().Add(10 * time.Second)
	for binds.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if binds.Load() == 0 {
		t.Fatal("bind was not called")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("Run error = %v, want %v", err, http.ErrServerClosed)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not stop")
	}
	if rebound == nil || !closed(rebound) {
		t.Fatal("listener returned by bind was not closed")
	}
}

func TestRunCancelDuringRestoreLogDoesNotStartNewServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initial := listen(t)
	rebound := &acceptCountingListener{Listener: listen(t)}
	defer rebound.Close()
	var state atomic.Int32
	state.Store(1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })}
	ready := func() (bool, error) { return state.Load() == 1, nil }
	bind := func(context.Context) (net.Listener, error) { return rebound, nil }
	logBlocked := make(chan struct{})
	unblockLog := make(chan struct{})
	logger := func(message string) {
		if message == "tailnet interface restored; listener rebound" {
			close(logBlocked)
			<-unblockLog
		}
	}
	var starts atomic.Int32
	start := func(server *http.Server, listener net.Listener, result chan<- error) {
		starts.Add(1)
		go func() { result <- server.Serve(listener) }()
	}
	done := make(chan error, 1)
	go func() { done <- run(ctx, server, initial, ready, bind, logger, start) }()
	getBody(t, initial.Addr().String(), "ok")
	if got := starts.Load(); got != 1 {
		t.Fatalf("server start calls after initial serve = %d, want 1", got)
	}
	state.Store(0)
	waitFor(t, func() bool { return closed(initial) })
	state.Store(1)
	select {
	case <-logBlocked:
	case <-time.After(10 * time.Second):
		t.Fatal("restored logger was not reached")
	}
	cancel()
	close(unblockLog)
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("Run error = %v, want %v", err, http.ErrServerClosed)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not stop")
	}
	if !closed(rebound) {
		t.Fatal("rebound listener was not closed; server may have started after cancellation")
	}
	if got := rebound.accepts.Load(); got != 0 {
		t.Fatalf("rebound listener Accept calls = %d, want 0 after cancellation", got)
	}
	if got := starts.Load(); got != 1 {
		t.Fatalf("server start calls after cancellation = %d, want 1", got)
	}
}

func TestRunCancelDuringBindErrorIsCleanShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initial := listen(t)
	var state atomic.Int32
	state.Store(1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })}
	ready := func() (bool, error) { return state.Load() == 1, nil }
	bindEntered := make(chan struct{})
	bind := func(ctx context.Context) (net.Listener, error) {
		close(bindEntered)
		<-ctx.Done()
		return nil, context.Canceled
	}
	done := make(chan error, 1)
	go func() { done <- Run(ctx, server, initial, ready, bind, nil) }()
	getBody(t, initial.Addr().String(), "ok")
	state.Store(0)
	waitFor(t, func() bool { return closed(initial) })
	state.Store(1)
	select {
	case <-bindEntered:
	case <-time.After(10 * time.Second):
		t.Fatal("bind was not called")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("Run error = %v, want %v", err, http.ErrServerClosed)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not stop")
	}
}

func TestCloneServerConfigPreservesExportedFields(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	connState := func(net.Conn, http.ConnState) {}
	baseContext := func(net.Listener) context.Context { return context.Background() }
	connContext := func(ctx context.Context, _ net.Conn) context.Context { return ctx }
	tlsConfig := &tls.Config{ServerName: "example.test"}
	http2 := &http.HTTP2Config{}
	protocols := &http.Protocols{}
	tlsNextProto := map[string]func(*http.Server, *tls.Conn, http.Handler){
		"h2": func(*http.Server, *tls.Conn, http.Handler) {},
	}
	source := &http.Server{
		Handler: handler, ReadTimeout: time.Second, ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout: 3 * time.Second, IdleTimeout: 4 * time.Second, MaxHeaderBytes: 1234,
		MaxHeaderValueCount: 17, ConnState: connState, ErrorLog: log.New(io.Discard, "test", 0),
		BaseContext: baseContext, ConnContext: connContext, HTTP2: http2, Protocols: protocols,
		DisableClientPriority: true, DisableGeneralOptionsHandler: true,
		TLSConfig: tlsConfig, TLSNextProto: tlsNextProto,
	}
	got := cloneServerConfig(source)
	if reflect.ValueOf(got.Handler).Pointer() != reflect.ValueOf(source.Handler).Pointer() || got.ReadTimeout != source.ReadTimeout || got.ReadHeaderTimeout != source.ReadHeaderTimeout ||
		got.WriteTimeout != source.WriteTimeout || got.IdleTimeout != source.IdleTimeout || got.MaxHeaderBytes != source.MaxHeaderBytes ||
		got.MaxHeaderValueCount != source.MaxHeaderValueCount || got.ErrorLog != source.ErrorLog ||
		got.DisableClientPriority != source.DisableClientPriority || got.DisableGeneralOptionsHandler != source.DisableGeneralOptionsHandler {
		t.Fatal("clone did not preserve scalar or interface fields")
	}
	if reflect.ValueOf(got.ConnState).Pointer() != reflect.ValueOf(source.ConnState).Pointer() ||
		reflect.ValueOf(got.BaseContext).Pointer() != reflect.ValueOf(source.BaseContext).Pointer() ||
		reflect.ValueOf(got.ConnContext).Pointer() != reflect.ValueOf(source.ConnContext).Pointer() {
		t.Fatal("clone did not preserve function fields")
	}
	if got.HTTP2 != source.HTTP2 || got.Protocols != source.Protocols || got.TLSConfig != source.TLSConfig {
		t.Fatal("clone did not preserve pointer fields")
	}
	if len(got.TLSNextProto) != 1 || got.TLSNextProto["h2"] == nil {
		t.Fatal("clone did not preserve TLSNextProto keys")
	}
}

type acceptCountingListener struct {
	net.Listener
	accepts atomic.Int32
}

func (l *acceptCountingListener) Accept() (net.Conn, error) {
	l.accepts.Add(1)
	return l.Listener.Accept()
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return l
}
func getBody(t *testing.T, address, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get("http://" + address)
		if err == nil {
			body, readErr := io.ReadAll(response.Body)
			response.Body.Close()
			if readErr == nil && string(body) == want {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("listener %s did not serve expected response", address)
}
func closed(l net.Listener) bool {
	c, err := net.DialTimeout("tcp", l.Addr().String(), 20*time.Millisecond)
	if err != nil {
		return true
	}
	c.Close()
	return false
}
func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not reached")
}

type testListener struct{ closed chan struct{} }

func (l *testListener) Accept() (net.Conn, error) { <-l.closed; return nil, net.ErrClosed }
func (l *testListener) Close() error {
	select {
	case <-l.closed:
	default:
		close(l.closed)
	}
	return nil
}
func (l *testListener) Addr() net.Addr { return testAddr("fake") }

type testAddr string

func (a testAddr) Network() string { return string(a) }
func (a testAddr) String() string  { return string(a) }
