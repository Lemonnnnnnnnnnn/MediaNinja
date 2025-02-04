package parsers

import (
	"media-crawler/core/request"
	"net/url"
	"strings"
)

// MediaType 表示媒体类型
type MediaType int

const (
	Image MediaType = iota
	Video
)

// MediaInfo 存储媒体信息
type MediaInfo struct {
	URL       *url.URL  `json:"url"`
	MediaType MediaType `json:"media_type"`
	Filename  string    `json:"filename"`
}

// ParseResult 存储解析结果
type ParseResult struct {
	Title   *string                `json:"title,omitempty"`
	Content *string                `json:"content,omitempty"`
	Media   []MediaInfo            `json:"media,omitempty"`
	Extra   map[string]interface{} `json:"extra,omitempty"`
}

// DefaultDownloader 提供默认的下载实现
type DefaultDownloader struct{}

// Download 默认的下载实现
func (d *DefaultDownloader) Download(client *request.Client, url string, filepath string) error {
	return client.DownloadFile(url, filepath, nil)
}

// Parser 定义解析器接口
type Parser interface {
	Parse(html string) (*ParseResult, error)
	GetDownloader() Downloader // 新增获取下载器的方法
}

// Downloader 定义下载器接口
type Downloader interface {
	Download(client *request.Client, url string, filepath string) error
}

func GetParser(url string) Parser {
	switch {
	case strings.Contains(url, "telegra.ph"):
		return &TelegraphParser{}
	case strings.Contains(url, "ddys"):
		return &DDYSParser{}
	default:
		return &DefaultParser{}
	}
}

type DefaultParser struct {
	DefaultDownloader
}

func (p *DefaultParser) GetDownloader() Downloader {
	return p
}

func (p *DefaultParser) Parse(html string) (*ParseResult, error) {
	// 实现默认的解析逻辑
	return &ParseResult{
		Extra: make(map[string]interface{}),
	}, nil
}
