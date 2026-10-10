package downloader

import (
	"MediaNinja/core/request/client"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestM3U8FailurePreservesResumeState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/list.m3u8":
			w.Write([]byte("#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXTINF:1,\none.ts\n#EXTINF:1,\ntwo.ts\n#EXT-X-ENDLIST\n"))
		case "/one.ts":
			w.Write([]byte("segment"))
		default:
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()
	output := filepath.Join(t.TempDir(), "video.mp4")
	m := NewM3U8Downloader(client.NewClient("", 1, 0), output, nil)
	if err := m.DownloadFromURL(srv.URL + "/list.m3u8"); err == nil {
		t.Fatal("failed segment accepted")
	}
	data, err := os.ReadFile(output + ".ts")
	if err != nil || string(data) != "segment" {
		t.Fatalf("partial data removed: %q %v", data, err)
	}
	state := m.loadDownloadState()
	if state == nil || !state.DownloadedSegments["one.ts"] || state.DownloadedSegments["two.ts"] {
		t.Fatalf("bad resume state: %+v", state)
	}
}
func TestVariantURLResolution(t *testing.T) {
	m := &M3U8Downloader{}
	resolved, err := m.buildVariantURL("../video/list.m3u8", "https://example.com/master/a.m3u8")
	if err != nil || resolved != "https://example.com/video/list.m3u8" {
		t.Fatalf("%s %v", resolved, err)
	}
}
