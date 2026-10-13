package crawler

import (
	"MediaNinja/core/config"
	"MediaNinja/core/events"
	"MediaNinja/core/parsers"
	"MediaNinja/core/request/client"
	"MediaNinja/utils/concurrent"
	"MediaNinja/utils/format"
	"MediaNinja/utils/io"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"path/filepath"
	"time"
)

type Crawler struct {
	observer  events.Observer
	eventMu   sync.Mutex
	client    *client.Client
	parser    parsers.Parser
	limiter   *concurrent.Limiter
	outputDir string
	// SkipFiles 列出无需重新下载的文件名（重试已收集文件时使用）。
	SkipFiles    map[string]bool
	currentTitle *string
	config       *config.Config
	ioManager    *io.Manager
}

// 添加元数据结构
type CrawlMetadata struct {
	EntryURL    string              `json:"entry_url"`
	Title       string              `json:"title,omitempty"`
	MediaCount  int                 `json:"media_count"`
	MediaInfos  []parsers.MediaInfo `json:"media_infos"`
	CrawledTime string              `json:"crawled_time"`
}

func NewCrawler(cfg *config.Config) *Crawler {
	copyCfg := *cfg
	return &Crawler{
		client:    client.NewClient(cfg.ProxyURL, cfg.MaxRetries, cfg.RetryDelay),
		limiter:   concurrent.NewLimiter(cfg.Concurrency),
		outputDir: cfg.OutputDir,
		config:    &copyCfg,
		ioManager: io.NewManager(cfg.OutputDir),
	}

}

type FileResult struct {
	File  string `json:"file"`
	Error string `json:"error,omitempty"`
}
type Result struct {
	Files []FileResult `json:"files"`
}

func (c *Crawler) emit(e events.Event) {
	c.eventMu.Lock()
	defer c.eventMu.Unlock()
	e.Time = time.Now()
	// 进度事件携带完整保存路径，文件结果事件携带基础文件名；统一为基础
	// 文件名，保证同一文件在任务 Progress 映射中是同一个键。
	if e.File != "" {
		e.File = e.File[strings.LastIndexAny(e.File, `/\`)+1:]
	}
	if c.observer != nil {
		c.observer(e)
	}
}
func (c *Crawler) Start(url string) error {
	_, err := c.Run(context.Background(), url, nil)
	return err
}
func (c *Crawler) Run(ctx context.Context, url string, observer events.Observer) (result Result, runErr error) {
	if err := c.config.Validate(); err != nil {
		return result, err
	}
	c.observer = observer
	c.client.Context = ctx
	c.client.Observer = c.emit
	c.emit(events.Event{Type: "analyzing"})

	c.parser = parsers.GetParserWithOptions(url, c.client, parsers.ListOptions{MaxPages: c.config.MaxPages, MaxVideos: c.config.MaxVideos, Concurrency: c.config.Concurrency})

	html, err := c.client.Get(url, nil)
	if err != nil {
		return result, fmt.Errorf("failed to fetch URL: %w", err)
	}

	resultParsed, err := c.parser.Parse(html)
	if err != nil {
		return result, fmt.Errorf("failed to parse HTML: %w", err)
	}

	c.currentTitle = resultParsed.Title

	var mu sync.Mutex
	var failures []error
	for _, message := range resultParsed.Failures {
		failures = append(failures, errors.New(message))
	}
	if len(resultParsed.Media) == 0 && len(resultParsed.Files) == 0 {
		failures = append(failures, fmt.Errorf("no media found"))
	}
	// 计划事件：解析完成后即可得知本轮要收集的文件总数，供任务统计使用。
	c.emit(events.Event{Type: "plan", Total: int64(len(resultParsed.Media) + len(resultParsed.Files))})
	record := func(file string, err error) {
		mu.Lock()
		defer mu.Unlock()
		r := FileResult{File: file}
		event := events.Event{Type: "file_completed", File: file}
		if err != nil {
			r.Error = err.Error()
			failures = append(failures, err)
			event.Type = "file_failed"
			event.Error = r.Error
		}
		result.Files = append(result.Files, r)
		c.emit(event)
	}
	if err := c.saveMetadata(url, resultParsed); err != nil {
		failures = append(failures, err)
	}
	for _, file := range resultParsed.Files {
		if c.SkipFiles[file.Filename] {
			record(file.Filename, nil)
			continue
		}
		record(file.Filename, c.writeFile(&file, "files"))
	}
	c.emit(events.Event{Type: "downloading"})
	for _, media := range resultParsed.Media {
		if ctx.Err() != nil {
			break
		}
		mediaInfo := media
		c.limiter.Execute(func() {
			if c.SkipFiles[mediaInfo.Filename] {
				record(mediaInfo.Filename, nil)
				return
			}
			record(mediaInfo.Filename, c.downloadMedia(&mediaInfo))
		})
	}
	c.limiter.Wait()
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return result, errors.Join(failures...)
}

// 添加获取标题目录的辅助方法
func (c *Crawler) getTitleDir() string {
	titleDir := "unnamed"
	if c.currentTitle != nil && *c.currentTitle != "" {
		titleDir = format.SanitizeWindowsPath(*c.currentTitle)
	}
	return titleDir
}

func (c *Crawler) downloadMedia(media *parsers.MediaInfo) error {
	if media.URL == nil {
		return fmt.Errorf("invalid media URL")
	}

	filename := media.Filename
	if filename == "" {
		filename = format.GetFileNameFromURL(media.URL.Path)
	}

	var subDir string
	switch media.MediaType {
	case parsers.Image:
		subDir = "images"
	case parsers.Video:
		subDir = "videos"
	default:
		subDir = "others"
	}

	savePath := filepath.Join(c.outputDir, c.getTitleDir(), subDir, format.SanitizeWindowsPath(filename))
	if err := c.ioManager.EnsureDir(savePath); err != nil {
		return err
	}

	err := c.parser.GetDownloader().Download(c.client, media.URL.String(), savePath)
	if err != nil {
		return err
	}

	return nil
}

// 添加保存元数据的方法
func (c *Crawler) saveMetadata(url string, result *parsers.ParseResult) error {
	metadata := CrawlMetadata{
		EntryURL:    url,
		MediaCount:  len(result.Media),
		MediaInfos:  result.Media,
		CrawledTime: time.Now().Format(time.RFC3339),
	}

	if result.Title != nil {
		metadata.Title = *result.Title
	}

	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	return c.ioManager.WriteFile(metadataJSON, "metadata.json", "", c.getTitleDir())
}

// Add new method to handle file writing
func (c *Crawler) writeFile(content *parsers.FileContent, subDir string) error {
	if content == nil {
		return fmt.Errorf("invalid file content")
	}

	if content.Data == nil {
		return fmt.Errorf("file content data is nil")
	}

	// Convert content.Data to string or []byte depending on type
	var dataToWrite interface{} = content.Data
	switch v := content.Data.(type) {
	case string:
		dataToWrite = v
	case []byte:
		dataToWrite = v
	default:
		return fmt.Errorf("unexpected content data type: %T", content.Data)
	}

	err := c.ioManager.WriteFile(dataToWrite, content.Filename, subDir, c.getTitleDir())
	if err != nil {
		return err
	}

	return nil
}
