package crawler

import (
	"fmt"
	"media-crawler/core/config"
	"media-crawler/core/parsers"
	"media-crawler/core/request"
	"media-crawler/utils/concurrent"
	"media-crawler/utils/format"
	"media-crawler/utils/logger"
	"os"
	"path/filepath"
)

type Crawler struct {
	client       *request.Client
	parser       parsers.Parser
	limiter      *concurrent.Limiter
	outputDir    string
	currentTitle *string
	config       *config.Config
}

func NewCrawler(cfg *config.Config) *Crawler {
	return &Crawler{
		client:    request.NewClient(cfg.ProxyURL),
		limiter:   concurrent.NewLimiter(cfg.Concurrency),
		outputDir: cfg.OutputDir,
		config:    cfg,
	}
}

func (c *Crawler) Start(url string) error {
	logger.Info("Starting crawler for URL: " + url)
	// 根据URL选择合适的解析器
	c.parser = parsers.GetParser(url)

	// 开始爬取
	html, err := c.client.GetHTML(url)
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

func (c *Crawler) downloadMedia(media *parsers.MediaInfo) {
	if media.URL == nil {
		logger.Error("Invalid media URL")
		return
	}

	// 构建保存路径
	filename := media.Filename
	if filename == "" {
		// 如果文件名为空，从URL中提取
		filename = format.GetFileNameFromURL(media.URL.Path)
	}

	// 根据媒体类型添加子目录
	var subDir string
	switch media.MediaType {
	case parsers.Image:
		subDir = "images"
	case parsers.Video:
		subDir = "videos"
	default:
		subDir = "others"
	}

	// 获取标题作为文件夹名称
	titleDir := "unnamed"
	if c.currentTitle != nil && *c.currentTitle != "" {
		titleDir = format.SanitizeWindowsPath(*c.currentTitle)
	}

	// 构建完整的保存路径
	savePath := filepath.Join(c.outputDir, titleDir, subDir, format.SanitizeWindowsPath(filename))

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(savePath), 0755); err != nil {
		logger.Error(fmt.Sprintf("Failed to create directory for %s: %v", filename, err))
		return
	}

	// 下载文件
	logger.Info(fmt.Sprintf("Downloading %s to %s", media.URL.String(), savePath))
	if err := c.client.DownloadFile(media.URL.String(), savePath); err != nil {
		logger.Error(fmt.Sprintf("Failed to download %s: %v", media.URL.String(), err))
		return
	}

	logger.Info(fmt.Sprintf("Successfully downloaded %s", filename))
}
