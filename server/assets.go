package server

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

//go:embed all:ui
var assets embed.FS

func embeddedFrontend() fs.FS { frontend, _ := fs.Sub(assets, "ui"); return frontend }

// SetFrontendDirectory is used only by the local development launcher.
func (s *Server) SetFrontendDirectory(directory string) error {
	frontend := os.DirFS(directory)
	if _, err := fs.Stat(frontend, "index.html"); err != nil {
		return err
	}
	s.frontend = frontend
	return nil
}
func (s *Server) setSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "medianinja_session", Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
}
func (s *Server) serveFrontend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(405)
		return
	}
	s.setSession(w)
	if _, err := fs.Stat(s.frontend, "index.html"); err != nil {
		http.Error(w, "Frontend is not built. Run pnpm build:web before starting the Web UI.", 503)
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" || name == "." {
		name = "index.html"
	}
	info, err := fs.Stat(s.frontend, name)
	if err != nil || info.IsDir() {
		// Missing assets must be 404s, not HTML. Application routes fall back to the SPA entry.
		if path.Ext(name) != "" || strings.HasPrefix(name, "assets/") {
			http.NotFound(w, r)
			return
		}
		name = "index.html"
	}
	request := r.Clone(r.Context())
	copyURL := *r.URL
	if name == "index.html" {
		copyURL.Path = "/"
	} else {
		copyURL.Path = "/" + name
	}
	request.URL = &copyURL
	http.FileServer(http.FS(s.frontend)).ServeHTTP(w, request)
}
