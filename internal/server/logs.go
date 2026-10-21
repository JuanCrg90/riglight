package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"riglight/internal/logs"
)

func validLogInput(w http.ResponseWriter, request *http.Request) bool {
	if !validRequestInput(w, request) || request.URL.ForceQuery || request.URL.RawPath != "" || len(request.Header.Values("Last-Event-ID")) != 0 {
		if request.URL.ForceQuery || request.URL.RawPath != "" || len(request.Header.Values("Last-Event-ID")) != 0 {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		}
		return false
	}
	return true
}

func (s *Server) logCatalog(w http.ResponseWriter, request *http.Request) {
	catalog, err := s.logSources.Catalog(request.Context())
	if err != nil || logs.ValidateCatalog(catalog) != nil {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE", "unavailable")
		return
	}
	body, err := json.Marshal(catalog)
	if err != nil || len(body) > logs.MaxCatalogBytes {
		writeError(w, http.StatusServiceUnavailable, "UNAVAILABLE", "unavailable")
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) logStream(w http.ResponseWriter, request *http.Request) {
	id := strings.TrimPrefix(request.URL.Path, "/api/v1/logs/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}
	// Refresh the catalog for every deliberate Start/Resume before any upstream
	// stream is opened. This also prevents a stale helper alias from rebinding.
	catalog, err := s.logSources.Catalog(request.Context())
	if err != nil || logs.ValidateCatalog(catalog) != nil || !catalogContains(catalog, id) || catalog.Mode == logs.ModeDisabled {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}
	select {
	case s.logSlots <- struct{}{}:
		defer func() { <-s.logSlots }()
	default:
		writeError(w, http.StatusTooManyRequests, "STREAM_LIMIT", "stream unavailable")
		return
	}
	provider, err := s.logSources.Provider(request.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}
	streamContext, cancel := context.WithTimeout(request.Context(), logs.SessionLifetime)
	stop := context.AfterFunc(s.context, cancel)
	defer stop()
	defer cancel()
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("X-Accel-Buffering", "no")
	if err := writeLogStatus(w, logs.Status{SchemaVersion: logs.SchemaVersion, Mode: catalog.Mode, SourceID: id, State: logs.StateOpening}); err != nil {
		return
	}
	sequence, ready := int64(0), false
	err = provider.Stream(streamContext, func() error {
		if ready {
			return logs.ErrInvalid
		}
		ready = true
		return writeLogStatus(w, logs.Status{SchemaVersion: logs.SchemaVersion, Mode: catalog.Mode, SourceID: id, State: logs.StateStreaming})
	}, func(stream logs.Stream, text string, truncated bool) error {
		if !ready || !utf8.ValidString(text) || len(text) > logs.MaxTextBytes {
			return logs.ErrInvalid
		}
		sequence++
		if sequence > logs.MaxEmittedLines {
			return logs.ErrLineLimit
		}
		return writeLogEvent(w, logs.Event{SchemaVersion: logs.SchemaVersion, Mode: catalog.Mode, SourceID: id, Seq: sequence, Text: text, Stream: stream, Truncated: truncated})
	})
	if request.Context().Err() != nil || s.context.Err() != nil {
		return
	}
	if errors.Is(streamContext.Err(), context.DeadlineExceeded) {
		err = logs.ErrSessionLimit
	}
	state, reason := logs.ReasonFor(err)
	_ = writeLogStatus(w, logs.Status{SchemaVersion: logs.SchemaVersion, Mode: catalog.Mode, SourceID: id, State: state, ReasonCode: reason})
}

func catalogContains(catalog logs.Catalog, id string) bool {
	for _, source := range catalog.Sources {
		if source.ID == id {
			return true
		}
	}
	return false
}
func writeLogStatus(w http.ResponseWriter, value logs.Status) error {
	return writeLogFrame(w, "log-status", value)
}
func writeLogEvent(w http.ResponseWriter, value logs.Event) error {
	return writeLogFrame(w, "log", value)
}
func writeLogFrame(w http.ResponseWriter, event string, value any) error {
	body, err := json.Marshal(value)
	if err != nil || len(body) > logs.MaxWireBytes {
		return logs.ErrInvalid
	}
	frame := fmt.Sprintf("event: %s\ndata: %s\n\n", event, body)
	if len(frame) > logs.MaxWireBytes {
		return logs.ErrInvalid
	}
	controller := http.NewResponseController(w)
	if err := controller.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		return err
	}
	if _, err := w.Write([]byte(frame)); err != nil {
		return err
	}
	if err := controller.Flush(); err != nil {
		return err
	}
	return controller.SetWriteDeadline(time.Time{})
}
