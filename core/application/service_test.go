package application

import (
	"MediaNinja/core/config"
	"MediaNinja/core/events"
	"MediaNinja/core/parsers"
	"MediaNinja/core/request/client"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
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

// testBatchSource 在测试中注册的批次源：第一候选 404，回退到 telegraph 页面，
// 用于验证批次抽象的回退语义与 SkipFiles 行为。
type testBatchSource struct{ base string }

func (s testBatchSource) Info() parsers.BatchSiteInfo {
	return parsers.BatchSiteInfo{ID: "test", Label: "TEST", Summary: "test source"}
}
func (s testBatchSource) Resolve(c *client.Client, query string) (string, error) {
	first, err := c.GetStream("GET", s.base+"/models/"+query+"/", nil, nil)
	if err != nil {
		return "", err
	}
	status := first.StatusCode
	first.Body.Close()
	if status == 404 {
		return s.base + "/telegra.ph", nil
	}
	return s.base + "/models/" + query + "/", nil
}

func TestBatchSourceFallbackAndSkipFiles(t *testing.T) {
	var base string
	fetches := map[string]int{}
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		fetches[r.URL.Path]++
		mu.Unlock()
		switch r.URL.Path {
		case "/telegra.ph":
			fmt.Fprintf(w, `<header><h1>Test</h1></header><img src="%s/good.jpg"><img src="%s/bad.jpg">`, base, base)
		case "/good.jpg":
			w.Write([]byte("image"))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	base = srv.URL
	parsers.RegisterBatchSource(testBatchSource{base: base})
	options := *config.New()
	options.OutputDir = t.TempDir()
	options.MaxRetries = 1
	options.RetryDelay = 0
	options.Concurrency = 1

	observer := func(events.Event) {}
	result, err := Execute(context.Background(), Request{Batch: &BatchRequest{Site: "test", Names: []string{"query"}}, Options: options}, observer)
	if err == nil || len(result.Files) != 2 {
		t.Fatalf("batch partial failure lost: %+v %v", result, err)
	}
	if _, err := parsers.GetBatchSource("missing"); err == nil {
		t.Fatal("unknown source accepted")
	}

	// 局部重试：跳过已成功的 001.jpg，bad.jpg 再次失败，good.jpg 不再发起下载。
	result, err = Execute(context.Background(), Request{Batch: &BatchRequest{Site: "test", Names: []string{"query"}}, SkipFiles: []string{"001.jpg"}, Options: options}, observer)
	if err == nil || len(result.Files) != 2 {
		t.Fatalf("retry result lost: %+v %v", result, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if fetches["/good.jpg"] != 1 {
		t.Fatalf("skipped file was downloaded again: %d", fetches["/good.jpg"])
	}
}
