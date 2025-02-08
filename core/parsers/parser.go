package parsers

import (
	"media-crawler/core/request"
	"net/url"
	"strings"
)

type MediaType int

const (
	Image MediaType = iota
	Video
	Subtitle
)

type MediaInfo struct {
	URL       *url.URL  `json:"url"`
	MediaType MediaType `json:"media_type"`
	Filename  string    `json:"filename"`
}

// FileContent represents content to be written to a file
type FileContent struct {
	Data        interface{} // can be []byte for binary or string for text
	Filename    string
	ContentType string // "binary" or "text"
}

type ParseResult struct {
	Title   *string                `json:"title,omitempty"`
	Content *string                `json:"content,omitempty"`
	Media   []MediaInfo            `json:"media,omitempty"`
	Extra   map[string]interface{} `json:"extra,omitempty"`
	Files   []FileContent          `json:"files,omitempty"` // New field for direct file contents
}

type Parser interface {
	Parse(html string) (*ParseResult, error)
	GetDownloader() Downloader
}

type Downloader interface {
	Download(client *request.Client, url string, filepath string) error
}

// DefaultDownloader 提供默认的下载实现
type DefaultDownloader struct{}

// Download 默认的下载实现
func (d *DefaultDownloader) Download(client *request.Client, url string, filepath string) error {
	return client.DownloadFile(url, filepath, nil)
}

func GetParser(url string, client *request.Client) Parser {
	switch {
	case strings.Contains(url, "telegra.ph"):
		return &TelegraphParser{}
	case strings.Contains(url, "ddys"):
		return NewDDYSParser(client)
	case strings.Contains(url, "ntdm"):
		return NewNTDMParser(client)
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
