package crawler

import (
	"encoding/json"
	"fmt"
	"media-crawler/core/config"
	"media-crawler/core/parsers"
	"media-crawler/core/request"
	"media-crawler/utils/concurrent"
	"media-crawler/utils/format"
	"media-crawler/utils/logger"
	"os"
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
	}
}

func (c *Crawler) Start(url string) error {
	logger.Info("Starting crawler for URL: " + url)
	// 根据URL选择合适的解析器
	c.parser = parsers.GetParser(url, c.client)

	// 开始爬取
	html, err := c.client.GetHTML(url, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch URL: %w", err)
	}

	// 解析内容
	result, err := c.parser.Parse(html)
	if err != nil {
		return fmt.Errorf("failed to parse HTML: %w", err)
	}

	// 保存标题
	c.currentTitle = result.Title

	// 如果有标题，记录日志
	if result.Title != nil {
		logger.Info("Parsed title: " + *result.Title)
	}

	// 保存元数据
	if err := c.saveMetadata(url, result); err != nil {
		logger.Error(fmt.Sprintf("Failed to save metadata: %v", err))
	}

	// 并行下载所有媒体文件
	for _, media := range result.Media {
		mediaInfo := media // 创建副本以避免闭包问题
		c.limiter.Execute(func() {
			c.downloadMedia(&mediaInfo)
		})
	}

	c.limiter.Wait() // 等待所有下载完成
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

// 添加创建目录的辅助方法
func (c *Crawler) ensureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0755)
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

	// 使用新的辅助方法
	savePath := filepath.Join(c.outputDir, c.getTitleDir(), subDir, format.SanitizeWindowsPath(filename))

	// 使用新的辅助方法
	if err := c.ensureDir(savePath); err != nil {
		logger.Error(fmt.Sprintf("Failed to create directory for %s: %v", filename, err))
		return
	}

	// 下载文件
	logger.Info(fmt.Sprintf("Starting download of %s", filename))

	// 使用 parser 的下载器进行下载
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

	// 使用新的辅助方法
	metadataPath := filepath.Join(c.outputDir, c.getTitleDir(), "metadata.json")

	// 使用新的辅助方法
	if err := c.ensureDir(metadataPath); err != nil {
		return fmt.Errorf("failed to create directory for metadata: %w", err)
	}

	// 将元数据转换为 JSON 并保存
	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metadataPath, metadataJSON, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	logger.Info("Saved metadata to: " + metadataPath)
	return nil
}
