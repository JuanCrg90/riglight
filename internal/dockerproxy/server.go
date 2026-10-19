package dockerproxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"riglight/internal/containers"
)

const helperPollInterval = 5 * time.Second

type HTTPServer struct {
	hub  *containers.Hub
	host string
	now  func() time.Time
}

func NewHTTPServer(hub *containers.Hub, host string, now func() time.Time) (*HTTPServer, error) {
	canonical, ok := canonicalLoopbackListen(host)
	if hub == nil || !ok || canonical != host {
		return nil, errors.New("invalid docker proxy HTTP server")
	}
	if now == nil {
		now = time.Now
	}
	return &HTTPServer{hub: hub, host: host, now: now}, nil
}
func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.headers(w)
	if r.Host != s.host || len(r.Header.Values("Origin")) != 0 {
		s.error(w, http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		s.error(w, http.StatusMethodNotAllowed)
		return
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery || r.ContentLength != 0 || r.TransferEncoding != nil {
		s.error(w, http.StatusBadRequest)
		return
	}
	switch r.URL.Path {
	case "/healthz":
		s.write(w, http.StatusOK, []byte(`{"status":"OK"}`))
	case "/api/v1/containers":
		body, err := s.hub.Latest().MarshalAt(s.now())
		if err != nil {
			s.error(w, http.StatusInternalServerError)
			return
		}
		s.write(w, http.StatusOK, body)
	default:
		s.error(w, http.StatusNotFound)
	}
}
func (s *HTTPServer) headers(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
}
func (s *HTTPServer) write(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
func (s *HTTPServer) error(w http.ResponseWriter, status int) {
	body, _ := json.Marshal(map[string]string{"error": "request failed"})
	s.write(w, status, body)
}

type snapshotCollector interface {
	Collect(context.Context) (containers.Sample, error)
}
type worker struct {
	hub       *containers.Hub
	collector snapshotCollector
	done      chan struct{}
	once      sync.Once
}

func newWorker(hub *containers.Hub, collector snapshotCollector) *worker {
	return &worker{hub: hub, collector: collector, done: make(chan struct{})}
}

// StartWorker starts the fixed-cadence collector outside request handlers.
func StartWorker(hub *containers.Hub, collector snapshotCollector, ctx context.Context) *worker {
	worker := newWorker(hub, collector)
	worker.Start(ctx)
	return worker
}
func (w *worker) Start(ctx context.Context) {
	w.once.Do(func() {
		go func() {
			defer close(w.done)
			w.collect(ctx)
			ticker := time.NewTicker(helperPollInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					w.collect(ctx)
				}
			}
		}()
	})
}
func (w *worker) collect(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	sample, err := w.collector.Collect(ctx)
	if err == nil && ctx.Err() == nil {
		_ = w.hub.Publish(sample)
	}
}
func (w *worker) Wait(ctx context.Context) {
	select {
	case <-w.done:
	case <-ctx.Done():
	}
}
