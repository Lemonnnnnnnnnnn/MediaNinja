package crawler

import (
	"encoding/json"
	"fmt"
	"media-crawler/core/config"
	"media-crawler/core/parsers"
	"media-crawler/core/request"
	"media-crawler/utils/concurrent"
	"media-crawler/utils/format"
	"media-crawler/utils/io"
	"media-crawler/utils/logger"
	"path/filepath"
	"time"
)

type Crawler struct {
	client       *request.Client
	parser       parsers.Parser
	limiter      *concurrent.Limiter
	outputDir    string
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
	return &Crawler{
		client:    request.NewClient(cfg.ProxyURL, cfg.MaxRetries, cfg.RetryDelay),
		limiter:   concurrent.NewLimiter(cfg.Concurrency),
		outputDir: cfg.OutputDir,
		config:    cfg,
		ioManager: io.NewManager(cfg.OutputDir),
	}
}

func (c *Crawler) Start(url string) error {
	logger.Info("Starting crawler for URL: " + url)
	c.parser = parsers.GetParser(url, c.client)

	html, err := c.client.Get(url, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch URL: %w", err)
	}

	result, err := c.parser.Parse(html)
	if err != nil {
		return fmt.Errorf("failed to parse HTML: %w", err)
	}

	c.currentTitle = result.Title

	if result.Title != nil {
		logger.Info("Parsed title: " + *result.Title)
	}

	// Save metadata
	if err := c.saveMetadata(url, result); err != nil {
		logger.Error(fmt.Sprintf("Failed to save metadata: %v", err))
	}

	// Handle direct file contents
	for _, file := range result.Files {
		subDir := "files" // Default subdirectory for direct file contents
		if err := c.writeFile(&file, subDir); err != nil {
			logger.Error(fmt.Sprintf("Failed to write file %s: %v", file.Filename, err))
		} else {
			logger.Info(fmt.Sprintf("Successfully wrote file: %s", file.Filename))
		}
	}

	// Handle media downloads
	for _, media := range result.Media {
		mediaInfo := media
		c.limiter.Execute(func() {
			c.downloadMedia(&mediaInfo)
		})
	}

	c.limiter.Wait()
	return nil
}

// 添加获取标题目录的辅助方法
func (c *Crawler) getTitleDir() string {
	titleDir := "unnamed"
	if c.currentTitle != nil && *c.currentTitle != "" {
		titleDir = format.SanitizeWindowsPath(*c.currentTitle)
	}
	return titleDir
}

func (c *Crawler) downloadMedia(media *parsers.MediaInfo) {
	if media.URL == nil {
		logger.Error("Invalid media URL")
		return
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
		logger.Error(fmt.Sprintf("Failed to create directory for %s: %v", filename, err))
		return
	}

	logger.Info(fmt.Sprintf("Starting download of %s", filename))

	err := c.parser.GetDownloader().Download(c.client, media.URL.String(), savePath)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to download %s: %v", media.URL.String(), err))
		return
	}

	logger.Info(fmt.Sprintf("Successfully downloaded %s", filename))
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

	logger.Info(fmt.Sprintf("Writing file %s to subdirectory %s", content.Filename, subDir))

	if content.Data == nil {
		logger.Error(fmt.Sprintf("File content data is nil for %s", content.Filename))
		return fmt.Errorf("file content data is nil")
	}

	// Convert content.Data to string or []byte depending on type
	var dataToWrite interface{} = content.Data
	switch v := content.Data.(type) {
	case string:
		logger.Info(fmt.Sprintf("Content is string type, length: %d", len(v)))
		dataToWrite = v
	case []byte:
		logger.Info(fmt.Sprintf("Content is []byte type, length: %d", len(v)))
		dataToWrite = v
	default:
		logger.Error(fmt.Sprintf("Unexpected content data type: %T", content.Data))
		return fmt.Errorf("unexpected content data type: %T", content.Data)
	}

	err := c.ioManager.WriteFile(dataToWrite, content.Filename, subDir, c.getTitleDir())
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to write file %s: %v", content.Filename, err))
		return err
	}

	logger.Info(fmt.Sprintf("Successfully wrote file %s", content.Filename))
	return nil
}
