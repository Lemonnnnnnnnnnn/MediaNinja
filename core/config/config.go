package config

import "fmt"

// Config 存储所有配置信息
type Config struct {
	URL         string // 目标URL
	ProxyURL    string // 代理URL
	Concurrency int    // 并发数
	OutputDir   string // 输出目录
	MaxRetries  int    // Add this field
	RetryDelay  int    // Add this field in seconds
	MaxPages    int    // rule34 列表页最多抓取的页数
	MaxVideos   int    // rule34 每个列表页最多下载的视频数，0 表示不限制
}

// New 创建默认配置
func New() *Config {
	return &Config{
		Concurrency: 5,           // 默认并发数
		OutputDir:   "downloads", // 默认下载目录
		MaxRetries:  3,           // Default value
		RetryDelay:  5,           // Default value in seconds
		MaxPages:    1,           // rule34 列表页默认只抓第一页
		MaxVideos:   0,           // rule34 默认不限制视频数
	}
}

// Validate prevents invalid worker counts and retry settings before execution.
func (c Config) Validate() error {
	if c.Concurrency < 1 || c.Concurrency > 64 {
		return fmt.Errorf("concurrency must be between 1 and 64")
	}
	if c.MaxRetries < 1 || c.MaxRetries > 20 {
		return fmt.Errorf("max retries must be between 1 and 20")
	}
	if c.RetryDelay < 0 || c.RetryDelay > 300 {
		return fmt.Errorf("retry delay must be between 0 and 300 seconds")
	}
	if c.MaxPages < 1 || c.MaxVideos < 0 {
		return fmt.Errorf("invalid list limits")
	}
	return nil
}
