package downloader

import (
	"MediaNinja/core/request/client"
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFFmpegResolution(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "medianinja-core.exe")
	bundled := filepath.Join(directory, "ffmpeg.exe")
	custom := filepath.Join(directory, "custom.exe")
	for _, path := range []string{bundled, custom} {
		if err := os.WriteFile(path, []byte("test"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	lookupCalls := 0
	lookup := func(string) (string, error) { lookupCalls++; return "system-ffmpeg", nil }
	path, err := resolveFFmpeg(custom, executable, lookup)
	if err != nil || path != custom || lookupCalls != 0 {
		t.Fatalf("explicit override not preferred: %s %v", path, err)
	}
	path, err = resolveFFmpeg("", executable, lookup)
	if err != nil || path != bundled || lookupCalls != 0 {
		t.Fatalf("bundled version not preferred: %s %v", path, err)
	}
	if _, err := resolveFFmpeg(filepath.Join(directory, "missing.exe"), executable, lookup); err == nil || lookupCalls != 0 {
		t.Fatal("invalid explicit path silently fell back")
	}
	if err := os.Remove(bundled); err != nil {
		t.Fatal(err)
	}
	path, err = resolveFFmpeg("", executable, lookup)
	if err != nil || path != "system-ffmpeg" || lookupCalls != 1 {
		t.Fatalf("PATH fallback failed: %s %v", path, err)
	}
	if _, err := resolveFFmpeg("", executable, func(string) (string, error) { return "", errors.New("not found") }); err == nil {
		t.Fatal("missing ffmpeg accepted")
	}
}

func TestBundledFFmpegRemux(t *testing.T) {
	if os.Getenv("MEDIANINJA_FFMPEG") == "" {
		t.Skip("pnpm test:ffmpeg runs this test with the bundled executable")
	}
	segment, err := os.ReadFile("../../../scripts/fixtures/remux.ts")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/video.m3u8":
			w.Write([]byte("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:1\n#EXTINF:1,\nsegment.ts\n#EXT-X-ENDLIST\n"))
		case "/segment.ts":
			w.Write(segment)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	output := filepath.Join(t.TempDir(), "video.mp4")
	m := NewM3U8Downloader(client.NewClient("", 1, 0), output, nil)
	if err := m.DownloadFromURL(srv.URL + "/video.m3u8"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil || len(data) < 8 || !bytes.Equal(data[4:8], []byte("ftyp")) {
		t.Fatalf("invalid MP4 output: %v", err)
	}
	for _, temporary := range []string{output + ".ts", m.getStateFilePath()} {
		if _, err := os.Stat(temporary); !os.IsNotExist(err) {
			t.Fatalf("successful remux did not clean %s: %v", temporary, err)
		}
	}
}
