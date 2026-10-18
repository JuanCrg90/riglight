package server

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

const contentSecurityPolicy = "default-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'"

func validateAuthority(authority string) error {
	host, port, err := net.SplitHostPort(authority)
	if err != nil || host == "" || port == "" || strings.ContainsAny(authority, " \t\r\n") {
		return errInvalidConfig
	}
	return nil
}

func validateOrigin(origin, authority string) error {
	parsed, err := parseOrigin(origin)
	if err != nil || parsed.Host != authority {
		return errInvalidConfig
	}
	return nil
}

func validateAdditionalOrigin(origin string) error {
	if _, err := parseOrigin(origin); err != nil {
		return errInvalidConfig
	}
	return nil
}

func parseOrigin(origin string) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(origin)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errInvalidConfig
	}
	return parsed, nil
}

func (s *Server) secure(w http.ResponseWriter, request *http.Request) bool {
	setSecurityHeaders(w.Header())
	if request.Host != s.allowedHost {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return false
	}
	origins := request.Header.Values("Origin")
	if len(origins) > 1 || (len(origins) == 1 && !s.originAllowed(origins[0])) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return false
	}
	return true
}

func validRequestInput(w http.ResponseWriter, request *http.Request) bool {
	if request.URL.RawQuery != "" || request.ContentLength != 0 || len(request.TransferEncoding) > 0 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return false
	}
	return true
}

func (s *Server) originAllowed(origin string) bool {
	_, ok := s.allowedOrigins[origin]
	return ok
}

func setSecurityHeaders(header http.Header) {
	header.Set("Cache-Control", "no-store")
	header.Set("Content-Security-Policy", contentSecurityPolicy)
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
}
