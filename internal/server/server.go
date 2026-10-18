// Package server exposes the loopback dashboard HTTP contract.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"riglight/internal/metrics"
)

var errInvalidConfig = errors.New("invalid server configuration")

type Config struct {
	Hub           *metrics.Hub
	AllowedHost   string
	AllowedOrigin string
	Now           func() time.Time
	Context       context.Context
}

// Server serves only the milestone-one health and sample APIs.
type Server struct {
	hub           *metrics.Hub
	allowedHost   string
	allowedOrigin string
	now           func() time.Time
	context       context.Context
	streamSlots   chan struct{}
}

func New(config Config) (*Server, error) {
	if config.Hub == nil || validateAuthority(config.AllowedHost) != nil || validateOrigin(config.AllowedOrigin, config.AllowedHost) != nil {
		return nil, errInvalidConfig
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.Context == nil {
		config.Context = context.Background()
	}
	return &Server{
		hub:           config.Hub,
		allowedHost:   config.AllowedHost,
		allowedOrigin: config.AllowedOrigin,
		now:           config.Now,
		context:       config.Context,
		streamSlots:   make(chan struct{}, maxSSEClients),
	}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	if !s.secure(w, request) {
		return
	}

	switch request.URL.Path {
	case "/healthz":
		if !requireGet(w, request) || !validRequestInput(w, request) {
			return
		}
		s.health(w)
	case "/api/v1/snapshot":
		if !requireGet(w, request) || !validRequestInput(w, request) {
			return
		}
		s.snapshot(w)
	case "/api/v1/events":
		if !requireGet(w, request) || !validRequestInput(w, request) {
			return
		}
		s.events(w, request)
	default:
		writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
	}
}

func requireGet(w http.ResponseWriter, request *http.Request) bool {
	if request.Method == http.MethodGet {
		return true
	}
	w.Header().Set("Allow", http.MethodGet)
	writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	return false
}

func (s *Server) health(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, []byte(`{"status":"OK"}`))
}

func (s *Server) snapshot(w http.ResponseWriter) {
	body, err := s.hub.Latest().MarshalAt(s.now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	body, err := json.Marshal(struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}{
		Error: struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{Code: code, Message: message},
	})
	if err != nil {
		return
	}
	writeJSON(w, status, body)
}

func writeJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
