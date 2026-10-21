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
	if !validLogSourceID(id) {
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
	// Admission precedes the helper-backed catalog refresh, so an overloaded
	// dashboard never opens another helper connection merely to reject it.
	catalog, err := s.logSources.Catalog(request.Context())
	if err != nil || logs.ValidateCatalog(catalog) != nil || !catalogContains(catalog, id) || catalog.Mode == logs.ModeDisabled {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}
	source, found := catalogSource(catalog, id)
	if !found {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
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
	sequence, targetRevision, ready := int64(0), int64(0), false
	var previousTarget logs.Target
	hasTarget := false
	readyStream := func() error {
		if ready {
			return logs.ErrInvalid
		}
		ready = true
		return writeLogStatus(w, logs.Status{SchemaVersion: logs.SchemaVersion, Mode: catalog.Mode, SourceID: id, State: logs.StateStreaming})
	}
	emitLine := func(stream logs.Stream, text string, truncated bool) error {
		if !ready || !utf8.ValidString(text) || len(text) > logs.MaxTextBytes {
			return logs.ErrInvalid
		}
		sequence++
		if sequence > logs.MaxEmittedLines {
			return logs.ErrLineLimit
		}
		return writeLogEvent(w, logs.Event{SchemaVersion: logs.SchemaVersion, Mode: catalog.Mode, SourceID: id, Seq: sequence, Text: text, Stream: stream, Truncated: truncated})
	}
	if targetProvider, ok := provider.(logs.TargetProvider); ok {
		err = targetProvider.StreamWithTarget(streamContext, readyStream, func(target logs.Target) error {
			if source.Kind != logs.KindLoadedModel || target.Validate() != nil || target.Mode != catalog.Mode || target.SourceID != id || (hasTarget && (target.Revision <= targetRevision || sameLogTarget(previousTarget, target))) {
				return logs.ErrInvalid
			}
			if err := writeLogTarget(w, target); err != nil {
				return err
			}
			previousTarget, targetRevision, hasTarget = target, target.Revision, true
			return nil
		}, emitLine)
	} else {
		err = provider.Stream(streamContext, readyStream, emitLine)
	}
	if request.Context().Err() != nil || s.context.Err() != nil {
		return
	}
	if errors.Is(streamContext.Err(), context.DeadlineExceeded) {
		err = logs.ErrSessionLimit
	}
	state, reason := logs.ReasonFor(err)
	_ = writeLogStatus(w, logs.Status{SchemaVersion: logs.SchemaVersion, Mode: catalog.Mode, SourceID: id, State: state, ReasonCode: reason})
}

func validLogSourceID(id string) bool {
	if id == "llama.proxy" {
		return true
	}
	for _, prefix := range []string{"model.", "docker."} {
		if strings.HasPrefix(id, prefix) && logs.ValidAlias(strings.TrimPrefix(id, prefix)) {
			return true
		}
	}
	return false
}

func catalogContains(catalog logs.Catalog, id string) bool {
	_, found := catalogSource(catalog, id)
	return found
}
func catalogSource(catalog logs.Catalog, id string) (logs.Source, bool) {
	for _, source := range catalog.Sources {
		if source.ID == id {
			return source, true
		}
	}
	return logs.Source{}, false
}
func sameLogTarget(left, right logs.Target) bool {
	if left.State != right.State || left.Model == nil || right.Model == nil {
		return left.State == right.State && left.Model == nil && right.Model == nil
	}
	return *left.Model == *right.Model
}

func writeLogStatus(w http.ResponseWriter, value logs.Status) error {
	return writeLogFrame(w, "log-status", value)
}
func writeLogEvent(w http.ResponseWriter, value logs.Event) error {
	return writeLogFrame(w, "log", value)
}
func writeLogTarget(w http.ResponseWriter, value logs.Target) error {
	return writeLogFrame(w, "log-target", value)
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
