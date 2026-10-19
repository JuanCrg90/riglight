package server

import (
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
)

const contentSecurityPolicy = "default-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'"

var (
	tailnetIPv4 = netip.MustParsePrefix("100.64.0.0/10")
	tailnetIPv6 = netip.MustParsePrefix("fd7a:115c:a1e0::/48")
)

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

func validateAllowedClients(clients []netip.Addr) (map[netip.Addr]struct{}, error) {
	if clients == nil {
		return nil, nil
	}
	if len(clients) == 0 {
		return nil, errInvalidConfig
	}
	allowed := make(map[netip.Addr]struct{}, len(clients))
	for _, client := range clients {
		client = client.Unmap()
		if !client.IsValid() || (!tailnetIPv4.Contains(client) && !tailnetIPv6.Contains(client)) {
			return nil, errInvalidConfig
		}
		allowed[client] = struct{}{}
	}
	return allowed, nil
}

func (s *Server) secure(w http.ResponseWriter, request *http.Request) bool {
	setSecurityHeaders(w.Header())
	if !s.clientAllowed(request.RemoteAddr) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return false
	}
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

func (s *Server) clientAllowed(remoteAddress string) bool {
	if s.allowedClients == nil && !s.allowAllTailnet {
		return true
	}
	client, ok := remoteTailnetAddr(remoteAddress)
	if !ok {
		return false
	}
	if s.allowAllTailnet {
		return tailnetIPv4.Contains(client) || tailnetIPv6.Contains(client)
	}
	_, ok = s.allowedClients[client]
	return ok
}

func remoteTailnetAddr(remoteAddress string) (netip.Addr, bool) {
	address, err := netip.ParseAddrPort(remoteAddress)
	if err != nil || address.Addr().Zone() != "" {
		return netip.Addr{}, false
	}
	return address.Addr().Unmap(), true
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
