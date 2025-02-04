package config

// Config 存储所有配置信息
type Config struct {
	URL         string // 目标URL
	ProxyURL    string // 代理URL
	Concurrency int    // 并发数
	OutputDir   string // 输出目录
}

// New 创建默认配置
func New() *Config {
	return &Config{
		Concurrency: 5,        // 默认并发数
		OutputDir:   "output", // 默认下载目录
	}
}
