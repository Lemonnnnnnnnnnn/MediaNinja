package parsers

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type TelegraphParser struct {
	DefaultDownloader
}

func (p *TelegraphParser) Parse(html string) (*ParseResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	result := &ParseResult{
		Media: make([]MediaInfo, 0),
		Extra: make(map[string]interface{}),
	}

	// 获取标题
	title := doc.Find("header h1").Text()
	if title != "" {
		result.Title = &title
	}

	// 查找所有图片
	doc.Find("img[src]").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists {
			return
		}

		// 处理图片URL
		imgURL, err := p.normalizeURL(src)
		if err != nil {
			return
		}

		// 生成文件名 (格式: 001.jpg, 002.jpg, ...)
		ext := path.Ext(imgURL.Path)
		if ext == "" {
			ext = ".jpg" // 默认扩展名
		}
		filename := fmt.Sprintf("%03d%s", i+1, ext)

		result.Media = append(result.Media, MediaInfo{
			URL:       imgURL,
			MediaType: Image,
			Filename:  filename,
		})
	})

	return result, nil
}

// normalizeURL 标准化图片URL
func (p *TelegraphParser) normalizeURL(src string) (*url.URL, error) {
	// 如果URL不是以http开头，添加协议
	if !strings.HasPrefix(src, "http") {
		src = "https://telegra.ph/" + strings.TrimPrefix(src, "//")
	}

	imgURL, err := url.Parse(src)
	if err != nil {
		return nil, err
	}

	return imgURL, nil
}

func (p *TelegraphParser) GetDownloader() Downloader {
	return p
}
