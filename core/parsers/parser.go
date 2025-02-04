package parsers

import (
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

// Parser 定义解析器接口
type Parser interface {
	Parse(html string) (*ParseResult, error)
}

func GetParser(url string) Parser {
	// 根据URL返回对应的解析器
	if strings.Contains(url, "telegra.ph") {
		return &TelegraphParser{}
	}
	return &DefaultParser{}
}

type DefaultParser struct{}

func (p *DefaultParser) Parse(html string) (*ParseResult, error) {
	// 实现默认的解析逻辑
	return &ParseResult{
		Extra: make(map[string]interface{}),
	}, nil
}
