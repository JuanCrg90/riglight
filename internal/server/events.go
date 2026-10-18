package server

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const (
	maxSSEClients = 8
	streamRetryMS = 2000
	writeTimeout  = 5 * time.Second
)

func (s *Server) events(w http.ResponseWriter, request *http.Request) {
	select {
	case s.streamSlots <- struct{}{}:
		defer func() { <-s.streamSlots }()
	default:
		writeError(w, http.StatusServiceUnavailable, "STREAM_LIMIT", "stream unavailable")
		return
	}

	subscription, err := s.hub.Subscribe()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error")
		return
	}
	defer subscription.Unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	streamContext, cancel := context.WithCancel(request.Context())
	stop := context.AfterFunc(s.context, cancel)
	defer stop()
	defer cancel()

	for {
		sample, nextErr := subscription.Next(streamContext)
		if nextErr != nil {
			return
		}
		if writeErr := s.writeSnapshotEvent(w, sample); writeErr != nil {
			return
		}
	}
}

func (s *Server) writeSnapshotEvent(w http.ResponseWriter, sample interface {
	MarshalAt(time.Time) ([]byte, error)
}) error {
	body, err := sample.MarshalAt(s.now())
	if err != nil {
		return err
	}
	frame := fmt.Sprintf("retry: %d\nevent: snapshot\ndata: %s\n\n", streamRetryMS, body)
	controller := http.NewResponseController(w)
	if err := controller.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
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
