package server

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

func (s *Server) assetPath(requestPath string) (string, bool) {
	if s.assets == nil {
		return "", false
	}
	if requestPath == "/" {
		return "index.html", true
	}
	if !strings.HasPrefix(requestPath, "/assets/") {
		return "", false
	}
	return strings.TrimPrefix(requestPath, "/"), true
}

func (s *Server) asset(w http.ResponseWriter, requestPath, name string) {
	if (requestPath != "/" && strings.HasSuffix(requestPath, "/")) || strings.HasSuffix(name, ".map") || !fs.ValidPath(name) || path.Clean(name) != name {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}
	info, err := fs.Stat(s.assets, name)
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}
	body, err := fs.ReadFile(s.assets, name)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
