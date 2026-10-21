package dockerproxy

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

type logCatalogWire struct {
	SchemaVersion int      `json:"schemaVersion"`
	Enabled       bool     `json:"enabled"`
	Aliases       []string `json:"aliases"`
}

func (s *HTTPServer) logCatalog(w http.ResponseWriter) {
	aliases := []string{}
	enabled := s.logProvider != nil
	if enabled {
		aliases = s.logProvider.Aliases()
	}
	body, err := json.Marshal(logCatalogWire{SchemaVersion: logs.SchemaVersion, Enabled: enabled, Aliases: aliases})
	if err != nil || len(body) > logs.MaxCatalogBytes {
		s.error(w, http.StatusInternalServerError)
		return
	}
	s.write(w, http.StatusOK, body)
}

func (s *HTTPServer) logs(w http.ResponseWriter, r *http.Request) {
	if s.logProvider == nil {
		s.error(w, http.StatusNotFound)
		return
	}
	if len(r.Header.Values("Last-Event-ID")) != 0 {
		s.error(w, http.StatusBadRequest)
		return
	}
	alias := strings.TrimPrefix(r.URL.Path, "/api/v1/logs/")
	if alias == "" || !validAlias(alias) {
		s.error(w, http.StatusNotFound)
		return
	}
	select {
	case s.logSlots <- struct{}{}:
		defer func() { <-s.logSlots }()
	default:
		s.error(w, http.StatusTooManyRequests)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), logs.SessionLifetime)
	defer cancel()
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("X-Accel-Buffering", "no")
	if err := writeLogStatus(w, logs.Status{SchemaVersion: logs.SchemaVersion, Mode: logs.ModeLive, SourceID: alias, State: logs.StateOpening}); err != nil {
		return
	}
	seq, started := int64(0), false
	err := s.logProvider.Stream(ctx, alias, func() error {
		if started {
			return logs.ErrInvalid
		}
		started = true
		return writeLogStatus(w, logs.Status{SchemaVersion: logs.SchemaVersion, Mode: logs.ModeLive, SourceID: alias, State: logs.StateStreaming})
	}, func(stream logs.Stream, text string, truncated bool) error {
		if !started || !utf8.ValidString(text) || len(text) > logs.MaxTextBytes {
			return logs.ErrInvalid
		}
		seq++
		return writeLogEvent(w, logs.Event{SchemaVersion: logs.SchemaVersion, Mode: logs.ModeLive, SourceID: alias, Seq: seq, Text: text, Stream: stream, Truncated: truncated})
	})
	if r.Context().Err() != nil {
		return
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		err = logs.ErrSessionLimit
	}
	state, reason := logs.ReasonFor(err)
	_ = writeLogStatus(w, logs.Status{SchemaVersion: logs.SchemaVersion, Mode: logs.ModeLive, SourceID: alias, State: state, ReasonCode: reason})
}

func writeLogStatus(w http.ResponseWriter, status logs.Status) error {
	return writeLogFrame(w, "log-status", status)
}
func writeLogEvent(w http.ResponseWriter, event logs.Event) error {
	return writeLogFrame(w, "log", event)
}
func writeLogFrame(w http.ResponseWriter, event string, value any) error {
	body, err := json.Marshal(value)
	if err != nil || len(body) > logs.MaxWireBytes {
		return logs.ErrInvalid
	}
	frame := fmt.Sprintf("event: %s\ndata: %s\n\n", event, body)
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
