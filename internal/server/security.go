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
	parsed, err := url.ParseRequestURI(origin)
	if err != nil || parsed.Scheme != "http" || parsed.Host != authority || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errInvalidConfig
	}
	return nil
}

func (s *Server) secure(w http.ResponseWriter, request *http.Request) bool {
	setSecurityHeaders(w.Header())
	if request.Host != s.allowedHost {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return false
	}
	origins := request.Header.Values("Origin")
	if len(origins) > 1 || (len(origins) == 1 && origins[0] != s.allowedOrigin) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return false
	}
	if request.URL.RawQuery != "" || request.ContentLength > 0 || len(request.TransferEncoding) > 0 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return false
	}
	return true
}

func setSecurityHeaders(header http.Header) {
	header.Set("Cache-Control", "no-store")
	header.Set("Content-Security-Policy", contentSecurityPolicy)
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
}
