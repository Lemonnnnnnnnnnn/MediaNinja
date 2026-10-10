package server

import (
	"MediaNinja/core/application"
	"MediaNinja/core/config"
	"MediaNinja/core/events"
	"MediaNinja/core/tasks"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestAPIAuthValidationAndOutputOwnership(t *testing.T) {
	m := tasks.New(func(ctx context.Context, r application.Request, o events.Observer) (application.Result, error) {
		return application.Result{}, nil
	})
	defer m.Close()
	defaults := *config.New()
	defaults.OutputDir = t.TempDir()
	s, err := New(m, "127.0.0.1:8080", defaults)
	s.frontend = fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>MediaNinja</title>")}, "assets/test.js": {Data: []byte("export {}")}}
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body, token, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "/api/tasks", "", "", ""); w.Code != 401 {
		t.Fatalf("unauthorized: %d", w.Code)
	}
	if w := call("POST", "/api/tasks", `{"url":"https://example.com"}`, s.Token(), "https://evil.example"); w.Code != 403 {
		t.Fatalf("origin accepted: %d", w.Code)
	}
	for _, body := range []string{`{"url":"file:///etc/passwd"}`, `{"url":"https://example.com","options":{"Concurrency":0}}`, `{"url":"https://example.com","unknown":1}`, `{"url":"https://example.com"} {}`} {
		if w := call("POST", "/api/tasks", body, s.Token(), ""); w.Code != 400 {
			t.Fatalf("body %s: %d %s", body, w.Code, w.Body.String())
		}
	}
	w := call("POST", "/api/tasks", `{"url":"https://example.com","options":{"OutputDir":"outside"}}`, s.Token(), "")
	if w.Code != 202 {
		t.Fatalf("submit: %d %s", w.Code, w.Body.String())
	}
	var task tasks.Task
	if err := json.Unmarshal(w.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	if task.Request.Options.OutputDir != defaults.OutputDir {
		t.Fatal("client changed output root")
	}
	if w := call("GET", "/api/tasks/"+task.ID, "", s.Token(), ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := call("POST", "/api/tasks/missing/cancel", "", s.Token(), ""); w.Code != 404 {
		t.Fatal(w.Code)
	}
	root := call("GET", "/", "", "", "")
	if root.Code != 200 || len(root.Result().Cookies()) != 1 {
		t.Fatal("UI session unavailable")
	}
	r := httptest.NewRequest("GET", "http://evil.example/api/tasks", nil)
	r.Header.Set("Authorization", "Bearer "+s.Token())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, r)
	if rec.Code != 403 {
		t.Fatal("invalid host accepted")
	}
}
func TestSSEInitialSnapshot(t *testing.T) {
	m := tasks.New(application.Execute)
	defer m.Close()
	s, _ := New(m, "localhost:8080", *config.New())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest("GET", "http://localhost:8080/api/events", nil).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer "+s.Token())
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "event: tasks\ndata: []") {
		t.Fatalf("bad SSE: %s", w.Body.String())
	}
}

func TestFrontendRoutingAndSessionBootstrap(t *testing.T) {
	m := tasks.New(application.Execute)
	defer m.Close()
	s, _ := New(m, "127.0.0.1:8080", *config.New())
	s.frontend = fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>React App</title>")}, "assets/test.js": {Data: []byte("export {}")}}
	for _, path := range []string{"/", "/settings", "/library/123", "/assets/test.js"} {
		r := httptest.NewRequest("GET", "http://127.0.0.1:8080"+path, nil)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	r := httptest.NewRequest("GET", "http://127.0.0.1:8080/assets/missing.js", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatalf("asset returned SPA: %d", w.Code)
	}
	r = httptest.NewRequest("GET", "http://127.0.0.1:8080/api/session", nil)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 200 || len(w.Result().Cookies()) != 1 || !w.Result().Cookies()[0].HttpOnly {
		t.Fatal("bootstrap did not establish an HttpOnly session")
	}
}
func TestConfigurationChangesOnlyAffectNewTasks(t *testing.T) {
	m := tasks.New(func(ctx context.Context, r application.Request, o events.Observer) (application.Result, error) {
		return application.Result{}, nil
	})
	defer m.Close()
	cfg := *config.New()
	cfg.OutputDir = t.TempDir()
	s, _ := New(m, "127.0.0.1:8080", cfg)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+s.Token())
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	var first tasks.Task
	w := call("POST", "/api/tasks", `{"url":"https://example.com"}`)
	json.Unmarshal(w.Body.Bytes(), &first)
	next := cfg
	next.Concurrency = 2
	next.OutputDir = t.TempDir()
	data, _ := json.Marshal(next)
	if w := call("PUT", "/api/config", string(data)); w.Code != 200 {
		t.Fatalf("config: %d %s", w.Code, w.Body.String())
	}
	previous, _ := m.Get(first.ID)
	if previous.Request.Options.Concurrency != cfg.Concurrency || previous.Request.Options.OutputDir != cfg.OutputDir {
		t.Fatal("existing task configuration changed")
	}
	var second tasks.Task
	w = call("POST", "/api/tasks", `{"url":"https://example.com"}`)
	json.Unmarshal(w.Body.Bytes(), &second)
	if second.Request.Options.Concurrency != 2 || second.Request.Options.OutputDir != next.OutputDir {
		t.Fatal("new task did not use updated config")
	}
	if w := call("PUT", "/api/config", `{"Concurrency":0}`); w.Code != 400 {
		t.Fatal("invalid config accepted")
	}
	if s.Config().Concurrency != 2 {
		t.Fatal("invalid config mutated defaults")
	}
}
