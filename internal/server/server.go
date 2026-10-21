// Package server exposes the loopback dashboard HTTP contract.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"riglight/internal/containers"
	"riglight/internal/gpu"
	"riglight/internal/llamaswap"
	"riglight/internal/logs"
	"riglight/internal/metrics"
	"riglight/internal/systemmetrics"
)

var errInvalidConfig = errors.New("invalid server configuration")

type Config struct {
	Hub               *metrics.Hub
	GPUHub            *gpu.Hub
	ModelsHub         *llamaswap.Hub
	ContainersHub     *containers.Hub
	SystemHub         *systemmetrics.Hub
	LogSources        logs.CatalogProvider
	AllowedHost       string
	AllowedOrigin     string
	AdditionalOrigins []string
	AllowedClients    []netip.Addr
	AllowAllTailnet   bool
	Now               func() time.Time
	Context           context.Context
	Assets            fs.FS
}

// Server serves only the milestone-one health and sample APIs.
type Server struct {
	hub             *metrics.Hub
	gpuHub          *gpu.Hub
	modelsHub       *llamaswap.Hub
	containersHub   *containers.Hub
	systemHub       *systemmetrics.Hub
	logSources      logs.CatalogProvider
	logSlots        chan struct{}
	allowedHost     string
	allowedOrigins  map[string]struct{}
	allowedClients  map[netip.Addr]struct{}
	allowAllTailnet bool
	now             func() time.Time
	context         context.Context
	assets          fs.FS
	streamSlots     chan struct{}
}

func New(config Config) (*Server, error) {
	if config.Hub == nil || validateAuthority(config.AllowedHost) != nil || validateOrigin(config.AllowedOrigin, config.AllowedHost) != nil {
		return nil, errInvalidConfig
	}
	allowedOrigins := map[string]struct{}{config.AllowedOrigin: {}}
	for _, origin := range config.AdditionalOrigins {
		if validateAdditionalOrigin(origin) != nil {
			return nil, errInvalidConfig
		}
		allowedOrigins[origin] = struct{}{}
	}
	if config.AllowAllTailnet && config.AllowedClients != nil {
		return nil, errInvalidConfig
	}
	allowedClients, err := validateAllowedClients(config.AllowedClients)
	if err != nil {
		return nil, errInvalidConfig
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.Context == nil {
		config.Context = context.Background()
	}
	assets := config.Assets
	if assets != nil {
		var err error
		assets, err = fs.Sub(assets, "dist")
		if err != nil {
			return nil, errInvalidConfig
		}
	}
	return &Server{
		hub:             config.Hub,
		gpuHub:          config.GPUHub,
		modelsHub:       config.ModelsHub,
		containersHub:   config.ContainersHub,
		systemHub:       config.SystemHub,
		logSources:      config.LogSources,
		logSlots:        make(chan struct{}, 2),
		allowedHost:     config.AllowedHost,
		allowedOrigins:  allowedOrigins,
		allowedClients:  allowedClients,
		allowAllTailnet: config.AllowAllTailnet,
		now:             config.Now,
		context:         config.Context,
		assets:          assets,
		streamSlots:     make(chan struct{}, maxSSEClients),
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
	case "/api/v1/gpus":
		if s.gpuHub == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
			return
		}
		if !requireGet(w, request) || !validRequestInput(w, request) {
			return
		}
		s.gpus(w)
	case "/api/v1/containers":
		if s.containersHub == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
			return
		}
		if !requireGet(w, request) || !validRequestInput(w, request) {
			return
		}
		s.containers(w)
	case "/api/v1/system":
		if s.systemHub == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
			return
		}
		if !requireGet(w, request) || !validRequestInput(w, request) {
			return
		}
		s.system(w)
	case "/api/v1/models":
		if s.modelsHub == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
			return
		}
		if !requireGet(w, request) || !validRequestInput(w, request) {
			return
		}
		s.models(w)
	case "/api/v1/log-sources":
		if s.logSources == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
			return
		}
		if !requireGet(w, request) || !validLogInput(w, request) {
			return
		}
		s.logCatalog(w, request)
	default:
		if strings.HasPrefix(request.URL.Path, "/api/v1/logs/") {
			if s.logSources == nil {
				writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
				return
			}
			if !requireGet(w, request) || !validLogInput(w, request) {
				return
			}
			s.logStream(w, request)
			return
		}
		name, ok := s.assetPath(request.URL.Path)
		if !ok {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
			return
		}
		if !requireGet(w, request) || !validRequestInput(w, request) {
			return
		}
		s.asset(w, request.URL.Path, name)
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

func (s *Server) gpus(w http.ResponseWriter) {
	body, err := s.gpuHub.Latest().MarshalAt(s.now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) containers(w http.ResponseWriter) {
	body, err := s.containersHub.Latest().MarshalAt(s.now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) system(w http.ResponseWriter) {
	body, err := s.systemHub.Latest().MarshalAt(s.now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) models(w http.ResponseWriter) {
	body, err := s.modelsHub.Latest().MarshalAt(s.now())
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
