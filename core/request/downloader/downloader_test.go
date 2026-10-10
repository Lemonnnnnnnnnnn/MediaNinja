package downloader

import (
	"MediaNinja/core/request/client"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServerIgnoresRangeTruncatesFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("new")) }))
	defer srv.Close()
	file := filepath.Join(t.TempDir(), "video.mp4")
	os.WriteFile(file, []byte("long incomplete old file"), 0600)
	if err := NewDownloader(client.NewClient("", 1, 0)).DownloadFile(srv.URL, file, nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(file)
	if string(data) != "new" {
		t.Fatalf("stale bytes retained: %q", data)
	}
}
func TestCancelDuringRetry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer srv.Close()
	c := client.NewClient("", 3, 30)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	c.Context = ctx
	start := time.Now()
	err := NewDownloader(c).DownloadFile(srv.URL, filepath.Join(t.TempDir(), "a"), nil)
	if err == nil || time.Since(start) > time.Second {
		t.Fatalf("retry ignored cancellation: %v", err)
	}
}
func TestUnknownLengthDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.(http.Flusher).Flush(); w.Write([]byte("chunked")) }))
	defer srv.Close()
	file := filepath.Join(t.TempDir(), "a")
	if err := NewDownloader(client.NewClient("", 1, 0)).DownloadFile(srv.URL, file, nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(file)
	if string(data) != "chunked" {
		t.Fatal(string(data))
	}
}
