package application

import (
	"MediaNinja/core/config"
	"MediaNinja/core/events"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestExecutionReportsPartialDownloadFailure(t *testing.T) {
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/telegra.ph":
			fmt.Fprintf(w, `<header><h1>Test</h1></header><img src="%s/good.jpg"><img src="%s/bad.jpg">`, base, base)
		case "/good.jpg":
			w.Write([]byte("image"))
		default:
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()
	base = srv.URL
	options := *config.New()
	options.OutputDir = t.TempDir()
	options.MaxRetries = 1
	options.RetryDelay = 0
	seen := map[string]bool{}
	result, err := Execute(context.Background(), Request{URL: base + "/telegra.ph", Options: options}, func(e events.Event) { seen[e.Type] = true })
	if err == nil || len(result.Files) != 2 {
		t.Fatalf("partial failure lost: %+v %v", result, err)
	}
	data, err := os.ReadFile(filepath.Join(options.OutputDir, "Test", "images", "001.jpg"))
	if err != nil || string(data) != "image" {
		t.Fatalf("file not saved: %s %v", data, err)
	}
	if !seen["progress"] || !seen["file_completed"] || !seen["file_failed"] {
		t.Fatalf("missing events: %v", seen)
	}
}
