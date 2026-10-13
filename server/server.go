// Package server adapts the task service to HTTP and serves the embedded UI.
package server

import (
	"MediaNinja/core/application"
	"MediaNinja/core/config"
	"MediaNinja/core/parsers"
	"MediaNinja/core/tasks"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Server struct {
	mu           sync.RWMutex
	frontend     fs.FS
	manager      *tasks.Manager
	token        string
	host         string
	defaults     config.Config
	settingsPath string
}

// New 创建服务。settingsPath 非空时启动加载持久化设置（无效则回退
// defaults），并在每次成功更新配置后原子落盘。
func New(m *tasks.Manager, host string, defaults config.Config, settingsPath string) (*Server, error) {
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return nil, err
	}
	if settingsPath != "" {
		if loaded, ok := config.Load(settingsPath); ok {
			defaults = loaded
		}
	}
	output, err := filepath.Abs(defaults.OutputDir)
	if err == nil {
		if err := os.MkdirAll(output, 0755); err == nil {
			defaults.OutputDir = output
		}
	}
	return &Server{manager: m, token: hex.EncodeToString(secret[:]), host: host, defaults: defaults, settingsPath: settingsPath, frontend: embeddedFrontend()}, nil
}
func (s *Server) Token() string { return s.token }
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self' ipc: http://ipc.localhost; frame-ancestors 'none'")
	if r.Host != s.host {
		fail(w, 403, "invalid host")
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != s.host || u.Scheme != "http" {
			fail(w, 403, "invalid origin")
			return
		}
	}

	if r.URL.Path == "/api/session" && r.Method == http.MethodGet {
		s.setSession(w)
		writeJSON(w, 200, map[string]any{"protocol": 1, "batch_sites": parsers.BatchSources()})
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		s.serveFrontend(w, r)
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if cookie, err := r.Cookie("medianinja_session"); err == nil && token == "" {
		token = cookie.Value
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) != 1 {
		fail(w, 401, "session or bearer token required")
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, s.Config()) })
	mux.HandleFunc("PUT /api/config", s.updateConfig)
	mux.HandleFunc("GET /api/tasks", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, s.manager.List()) })
	mux.HandleFunc("POST /api/tasks", s.submit)
	mux.HandleFunc("GET /api/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		t, ok := s.manager.Get(r.PathValue("id"))
		if !ok {
			fail(w, 404, "task not found")
			return
		}
		writeJSON(w, 200, t)
	})
	mux.HandleFunc("POST /api/tasks/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		if !s.manager.Cancel(r.PathValue("id")) {
			fail(w, 404, "task not found")
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("POST /api/tasks/{id}/retry", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Scope string `json:"scope"`
		}
		if r.ContentLength != 0 {
			d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
			d.DisallowUnknownFields()
			if err := d.Decode(&body); err != nil {
				fail(w, 400, err.Error())
				return
			}
		}
		t, err := s.manager.Retry(r.PathValue("id"), body.Scope)
		if err != nil {
			fail(w, retryStatus(err), err.Error())
			return
		}
		writeJSON(w, 202, t)
	})
	mux.HandleFunc("DELETE /api/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.manager.Remove(r.PathValue("id")); err != nil {
			if errors.Is(err, tasks.ErrTaskActive) {
				fail(w, 409, err.Error())
			} else {
				fail(w, 404, err.Error())
			}
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("DELETE /api/tasks", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]int{"removed": s.manager.ClearFinished()})
	})
	mux.HandleFunc("GET /api/events", s.events)
	mux.ServeHTTP(w, r)
}

// retryStatus 把任务服务的可预期错误映射为 HTTP 状态码。
func retryStatus(err error) int {
	switch {
	case errors.Is(err, tasks.ErrTaskNotFound):
		return 404
	case errors.Is(err, tasks.ErrTaskActive), errors.Is(err, tasks.ErrNothingFailed):
		return 409
	default:
		return 400
	}
}
func (s *Server) submit(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		fail(w, 415, "application/json required")
		return
	}
	settings := s.Config()
	req := struct {
		URL     string                    `json:"url"`
		Batch   *application.BatchRequest `json:"batch"`
		Options config.Config             `json:"options"`
	}{Options: settings}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	d.DisallowUnknownFields()
	if err := d.Decode(&req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if err := d.Decode(new(any)); err != io.EOF {
		fail(w, 400, "expected one JSON value")
		return
	}
	// Output location is owned by the server, never selected by an API caller.
	req.Options.OutputDir = settings.OutputDir
	request := application.Request{URL: req.URL, Batch: req.Batch, Options: req.Options}
	if err := request.Validate(); err != nil {
		fail(w, 400, err.Error())
		return
	}
	t, err := s.manager.Submit(request)
	if err != nil {
		fail(w, 503, err.Error())
		return
	}
	writeJSON(w, 202, t)
}
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		fail(w, 500, "streaming unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	send := func() bool {
		http.NewResponseController(w).SetWriteDeadline(time.Now().Add(10 * time.Second))
		data, _ := json.Marshal(s.manager.List())
		_, err := fmt.Fprintf(w, "event: tasks\ndata: %s\n\n", data)
		f.Flush()
		return err == nil
	}
	if !send() {
		return
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if !send() {
				return
			}
		}
	}
}

// Config returns a copy so each new task keeps an independent configuration snapshot.
func (s *Server) Config() config.Config { s.mu.RLock(); defer s.mu.RUnlock(); return s.defaults }
func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		fail(w, 415, "application/json required")
		return
	}
	settings := s.Config()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&settings); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		fail(w, 400, "expected one JSON value")
		return
	}
	if err := (application.Request{URL: "https://example.com", Options: settings}).Validate(); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if strings.TrimSpace(settings.OutputDir) == "" {
		fail(w, 400, "output directory is required")
		return
	}
	output, err := filepath.Abs(settings.OutputDir)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		fail(w, 400, err.Error())
		return
	}
	settings.OutputDir = output
	if err := config.Save(s.settingsPath, settings); err != nil {
		fail(w, 500, "settings applied but could not be persisted: "+err.Error())
		return
	}
	s.mu.Lock()
	s.defaults = settings
	s.mu.Unlock()
	writeJSON(w, 200, settings)
}
