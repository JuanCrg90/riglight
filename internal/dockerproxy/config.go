// Package dockerproxy implements the filtering Docker socket boundary.
package dockerproxy

import (
	"errors"
	"net"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultListen = "127.0.0.1:9123"

type Target struct {
	Alias     string
	Reference string
}
type Config struct {
	Socket     string
	Listen     string
	Containers []Target
	Logs       bool
}

func NewConfig(socket, listen string, selections []string) (Config, error) {
	if !filepath.IsAbs(socket) || socket == "/" || len(selections) == 0 || len(selections) > 16 {
		return Config{}, errors.New("invalid docker proxy configuration")
	}
	if listen == "" {
		listen = defaultListen
	}
	canonicalListen, ok := canonicalLoopbackListen(listen)
	if !ok {
		return Config{}, errors.New("invalid docker proxy configuration")
	}
	config := Config{Socket: socket, Listen: canonicalListen, Containers: make([]Target, 0, len(selections))}
	aliases, references := map[string]struct{}{}, map[string]struct{}{}
	for _, selection := range selections {
		alias, ref, ok := strings.Cut(selection, "=")
		if !ok || !validAlias(alias) || !validReference(ref) {
			return Config{}, errors.New("invalid docker proxy configuration")
		}
		if _, exists := aliases[alias]; exists {
			return Config{}, errors.New("invalid docker proxy configuration")
		}
		if _, exists := references[ref]; exists {
			return Config{}, errors.New("invalid docker proxy configuration")
		}
		aliases[alias], references[ref] = struct{}{}, struct{}{}
		config.Containers = append(config.Containers, Target{Alias: alias, Reference: ref})
	}
	return config, nil
}
func canonicalLoopbackListen(address string) (string, bool) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", false
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", false
	}
	value, err := strconv.Atoi(port)
	if err != nil || value <= 0 || value > 65535 {
		return "", false
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(value)), true
}
func validAlias(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for i := range value {
		c := value[i]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || (i > 0 && (c == '_' || c == '.' || c == '-'))) {
			return false
		}
	}
	return true
}
func validReference(value string) bool {
	if len(value) == 0 || len(value) > 128 || value == "." || value == ".." {
		return false
	}
	for _, c := range value {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '.' || c == '-') {
			return false
		}
	}
	return true
}
