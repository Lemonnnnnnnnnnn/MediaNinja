package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"MediaNinja/core/request/types"
)

type RequestOption = types.RequestOption
type ClientInterface = types.ClientInterface

type Downloader struct {
	client ClientInterface

	maxRetries int
	retryDelay time.Duration

	showProgress bool
}

func NewDownloader(client ClientInterface, showProgress bool) *Downloader {
	return &Downloader{
		client:       client,
		maxRetries:   client.GetMaxRetries(),
		retryDelay:   client.GetRetryDelay(),
		showProgress: showProgress,
	}
}

// DownloadFile downloads a file from URL to the specified filepath
func (d *Downloader) DownloadFile(url string, filepath string, opts *RequestOption) error {
	if err := os.MkdirAll(path.Dir(filepath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fmt.Println("DownloadFile", url)
	// 检查是否是 M3U8 文件
	if strings.Contains(strings.ToLower(url), ".m3u8") {
		fmt.Println("DownloadFile is M3U8")
		// 使用新的 M3U8 下载器，传入 showProgress 参数
		m3u8Downloader := NewM3U8Downloader(d.client, filepath, opts, d.showProgress)
		return m3u8Downloader.DownloadFromURL(url)
	}

	return d.downloadRegularFile(url, filepath, opts)
}

// DownloadFileWithPrefix downloads a file from URL to the specified filepath with URL prefix support
func (d *Downloader) DownloadFileWithPrefix(url string, filepath string, opts *RequestOption, urlPrefix string) error {
	if err := os.MkdirAll(path.Dir(filepath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fmt.Println("DownloadFileWithPrefix", url, "prefix:", urlPrefix)
	// 检查是否是 M3U8 文件
	if strings.Contains(strings.ToLower(url), ".m3u8") {
		fmt.Println("DownloadFileWithPrefix is M3U8")
		// 使用带前缀的 M3U8 下载器
		m3u8Downloader := NewM3U8DownloaderWithPrefix(d.client, filepath, opts, d.showProgress, urlPrefix)
		return m3u8Downloader.DownloadFromURL(url)
	}

	return d.downloadRegularFile(url, filepath, opts)
}

func (d *Downloader) downloadRegularFile(url string, filepath string, opts *RequestOption) error {
	var attempt int
	for attempt = 0; attempt < d.maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(d.retryDelay)
		}

		// Get file info for resume download
		var startPos int64 = 0
		fi, err := os.Stat(filepath)
		if err == nil {
			startPos = fi.Size()
		}

		// Prepare headers for resume
		var headers map[string]string
		if startPos > 0 {
			headers = map[string]string{
				"Range": fmt.Sprintf("bytes=%d-", startPos),
			}
		}

		// Get response stream
		resp, err := d.client.GetStream("GET", url, opts, headers)
		if err != nil {
			if attempt < d.maxRetries-1 {
				fmt.Printf("Download attempt %d failed: %v, retrying...\n", attempt+1, err)
				continue
			}
			return fmt.Errorf("failed to send request after %d attempts: %w", d.maxRetries, err)
		}
		defer resp.Body.Close()

		// Check response status
		switch resp.StatusCode {
		case http.StatusOK:
			startPos = 0
		case http.StatusPartialContent:
			// Server supports resume
		case http.StatusRequestedRangeNotSatisfiable:
			return nil
		default:
			if attempt < d.maxRetries-1 {
				fmt.Printf("Download attempt %d failed with status code %d, retrying...\n", attempt+1, resp.StatusCode)
				continue
			}
			return fmt.Errorf("unexpected status code after %d attempts: %d", d.maxRetries, resp.StatusCode)
		}

		if success := d.processDownload(resp, filepath, startPos); success {
			return nil
		}
	}

	return fmt.Errorf("download failed after %d attempts", d.maxRetries)
}

func (d *Downloader) processDownload(resp *http.Response, filepath string, startPos int64) bool {
	// Get file total size
	contentLength := resp.ContentLength
	if contentLength <= 0 {
		return false
	}
	totalSize := startPos + contentLength

	// Open or create file
	flags := os.O_CREATE | os.O_WRONLY
	if startPos > 0 {
		flags |= os.O_APPEND
	}
	file, err := os.OpenFile(filepath, flags, 0644)
	if err != nil {
		return false
	}
	defer file.Close()

	// Create progress tracker only if showProgress is true
	var progress *DownloadProgress
	if d.showProgress {
		progress = NewDownloadProgress(path.Base(filepath), totalSize, startPos)
	}

	// Use buffered reading for better performance
	bufSize := 32 * 1024 // 32KB buffer
	buf := make([]byte, bufSize)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := file.Write(buf[:n]); werr != nil {
				if progress != nil {
					progress.Fail(werr)
				}
				return false
			}
			if progress != nil {
				progress.Update(int64(n))
			}
		}
		if err == io.EOF {
			if progress != nil {
				progress.Success()
			}
			return true
		}
		if err != nil {
			if progress != nil {
				progress.Fail(err)
			}
			return false
		}
	}
}
