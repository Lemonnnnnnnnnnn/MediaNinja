package parsers

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Rule34VideoParser struct {
	DefaultDownloader
}

func (p *Rule34VideoParser) Parse(html string) (*ParseResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	result := &ParseResult{
		Media: make([]MediaInfo, 0),
		Extra: make(map[string]interface{}),
	}

	// 从title标签获取标题
	title := doc.Find("title").Text()
	if title != "" {
		result.Title = &title
	}

	// 解析下载链接: #tab_video_info > div:nth-child(4) > div 的第一个 a 标签
	downloadElement := doc.Find("#tab_video_info > div:nth-child(4) > div a").First()
	if downloadElement.Length() == 0 {
		return result, fmt.Errorf("download link not found")
	}

	href, exists := downloadElement.Attr("href")
	if !exists || href == "" {
		return result, fmt.Errorf("download link href not found")
	}

	// 处理下载URL
	videoURL, err := p.normalizeURL(href)
	if err != nil {
		return result, fmt.Errorf("failed to normalize download URL: %w", err)
	}

	// 生成文件名
	ext := path.Ext(videoURL.Path)
	if ext == "" {
		ext = ".mp4" // 默认视频扩展名
	}

	// 使用标题作为文件名（需要清理非法字符）
	safeTitle := p.sanitizeFilename(title)
	if safeTitle == "" {
		safeTitle = "video"
	}
	filename := safeTitle + ext

	result.Media = append(result.Media, MediaInfo{
		URL:       videoURL,
		MediaType: Video,
		Filename:  filename,
	})

	return result, nil
}

// normalizeURL 标准化视频URL
func (p *Rule34VideoParser) normalizeURL(src string) (*url.URL, error) {
	// 如果URL不是以http开头，可能是相对路径
	if !strings.HasPrefix(src, "http") {
		// 这里需要根据实际情况确定base URL
		// 暂时假设是相对路径，添加https协议
		if strings.HasPrefix(src, "//") {
			src = "https:" + src
		} else if strings.HasPrefix(src, "/") {
			src = "https://rule34video.com" + src
		} else {
			src = "https://rule34video.com/" + src
		}
	}

	videoURL, err := url.Parse(src)
	if err != nil {
		return nil, err
	}

	return videoURL, nil
}

// sanitizeFilename 清理文件名中的非法字符
func (p *Rule34VideoParser) sanitizeFilename(filename string) string {
	// 移除或替换文件名中的非法字符
	replacements := map[string]string{
		"<": "_",
		">": "_",
		":": "_",
		"\"": "_",
		"/": "_",
		"\\": "_",
		"|": "_",
		"?": "_",
		"*": "_",
	}

	sanitized := filename
	for old, new := range replacements {
		sanitized = strings.ReplaceAll(sanitized, old, new)
	}

	// 移除多余的空格和特殊字符
	sanitized = strings.TrimSpace(sanitized)

	// 如果文件名过长，截断
	if len(sanitized) > 100 {
		sanitized = sanitized[:100]
	}

	return sanitized
}

func (p *Rule34VideoParser) GetDownloader() Downloader {
	return p
}