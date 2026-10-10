package downloader

import (
	"MediaNinja/core/events"
	"MediaNinja/core/request/types"
	"fmt"
	"io"
	"net/http"
	"os"
	filepathpkg "path/filepath"
	"strings"
	"time"
)

type RequestOption = types.RequestOption
type ClientInterface = types.ClientInterface
type Downloader struct {
	client     ClientInterface
	maxRetries int
	retryDelay time.Duration
}

func NewDownloader(client ClientInterface) *Downloader {
	return &Downloader{client: client, maxRetries: client.GetMaxRetries(), retryDelay: client.GetRetryDelay()}
}
func (d *Downloader) DownloadFile(url, filepath string, opts *RequestOption) error {
	return d.DownloadFileWithPrefix(url, filepath, opts, "")
}
func (d *Downloader) DownloadFileWithPrefix(url, filepath string, opts *RequestOption, prefix string) error {
	if err := os.MkdirAll(filepathpkg.Dir(filepath), 0755); err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(url), ".m3u8") {
		return NewM3U8DownloaderWithPrefix(d.client, filepath, opts, prefix).DownloadFromURL(url)
	}
	return d.downloadRegularFile(url, filepath, opts)
}
func (d *Downloader) downloadRegularFile(url, filepath string, opts *RequestOption) error {
	var lastErr error
	for attempt := 0; attempt < d.maxRetries; attempt++ {
		if attempt > 0 {
			if err := waitRetry(d.client, d.retryDelay); err != nil {
				return err
			}
		}
		if err := downloadContext(d.client).Err(); err != nil {
			return err
		}
		var start int64
		if info, err := os.Stat(filepath); err == nil {
			start = info.Size()
		} else if !os.IsNotExist(err) {
			return err
		}
		headers := map[string]string{}
		if start > 0 {
			headers["Range"] = fmt.Sprintf("bytes=%d-", start)
		}
		response, err := d.client.GetStream("GET", url, opts, headers)
		if err != nil {
			lastErr = err
			continue
		}
		switch response.StatusCode {
		case http.StatusOK:
			start = 0
		case http.StatusPartialContent:
			var offset, end, total int64
			_, err := fmt.Sscanf(response.Header.Get("Content-Range"), "bytes %d-%d/%d", &offset, &end, &total)
			if err != nil || offset != start || end < offset || total <= end {
				response.Body.Close()
				return fmt.Errorf("invalid Content-Range for resume")
			}
		case http.StatusRequestedRangeNotSatisfiable:
			var total int64
			_, err := fmt.Sscanf(response.Header.Get("Content-Range"), "bytes */%d", &total)
			response.Body.Close()
			if err == nil && start > 0 && start == total {
				report(d.client, events.Event{Type: "progress", File: filepath, Completed: start, Total: total, Unit: "bytes"})
				return nil
			}
			return fmt.Errorf("range not satisfiable; completion cannot be verified")
		default:
			lastErr = fmt.Errorf("unexpected HTTP status: %d", response.StatusCode)
			response.Body.Close()
			continue
		}
		lastErr = d.processDownload(response, filepath, start)
		response.Body.Close()
		if lastErr == nil {
			return nil
		}
	}
	return fmt.Errorf("download failed after %d attempts: %w", d.maxRetries, lastErr)
}
func (d *Downloader) processDownload(response *http.Response, filepath string, start int64) error {
	total := int64(0)
	if response.ContentLength >= 0 {
		total = start + response.ContentLength
	}
	flags := os.O_CREATE | os.O_WRONLY
	if start > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	file, err := os.OpenFile(filepath, flags, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	completed := start
	emit := func() {
		report(d.client, events.Event{Type: "progress", File: filepath, Completed: completed, Total: total, Unit: "bytes"})
	}
	emit()
	last := time.Now()
	buffer := make([]byte, 32*1024)
	for {
		if err := downloadContext(d.client).Err(); err != nil {
			return err
		}
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			written, err := file.Write(buffer[:n])
			if err != nil {
				return err
			}
			if written != n {
				return io.ErrShortWrite
			}
			completed += int64(n)
		}
		if time.Since(last) >= 200*time.Millisecond {
			emit()
			last = time.Now()
		}
		if readErr == io.EOF {
			emit()
			if response.ContentLength >= 0 && completed != total {
				return io.ErrUnexpectedEOF
			}
			return file.Close()
		}
		if readErr != nil {
			return readErr
		}
	}
}
