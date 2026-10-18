package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"riglight/internal/gpu"
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

	cpuSubscription, err := s.hub.Subscribe()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error")
		return
	}
	defer cpuSubscription.Unsubscribe()
	var gpuUpdates <-chan gpu.Sample
	var unsubscribeGPU func()
	if s.gpuHub != nil {
		gpuSubscription, subscribeErr := s.gpuHub.Subscribe()
		if subscribeErr != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error")
			return
		}
		gpuUpdates = gpuSubscription.Updates()
		unsubscribeGPU = gpuSubscription.Unsubscribe
	}
	if unsubscribeGPU != nil {
		defer unsubscribeGPU()
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	streamContext, cancel := context.WithCancel(request.Context())
	stop := context.AfterFunc(s.context, cancel)
	defer stop()
	defer cancel()

	initialCPU, nextErr := cpuSubscription.Next(streamContext)
	if nextErr != nil || s.writeEvent(w, "snapshot", initialCPU) != nil {
		return
	}
	if gpuUpdates != nil {
		select {
		case initialGPU := <-gpuUpdates:
			if s.writeEvent(w, "gpus", initialGPU) != nil {
				return
			}
		case <-streamContext.Done():
			return
		}
	}
	for {
		select {
		case sample := <-cpuSubscription.Updates():
			if s.writeEvent(w, "snapshot", sample) != nil {
				return
			}
		case sample := <-gpuUpdates:
			if s.writeEvent(w, "gpus", sample) != nil {
				return
			}
		case <-streamContext.Done():
			return
		}
	}
}

type gpuSample interface {
	MarshalAt(time.Time) ([]byte, error)
}

func (s *Server) writeSnapshotEvent(w http.ResponseWriter, sample interface {
	MarshalAt(time.Time) ([]byte, error)
}) error {
	return s.writeEvent(w, "snapshot", sample)
}

func (s *Server) writeEvent(w http.ResponseWriter, event string, sample gpuSample) error {
	body, err := sample.MarshalAt(s.now())
	if err != nil {
		return err
	}
	frame := fmt.Sprintf("retry: %d\nevent: %s\ndata: %s\n\n", streamRetryMS, event, body)
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
