package parsers

import (
	"encoding/json"
	"fmt"
	"media-crawler/core/request"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type DDYSParser struct {
	downloader DDYSDownloader
}

type DDYSDownloader struct{}

func (p *DDYSParser) GetDownloader() Downloader {
	return &p.downloader
}

func (p *DDYSParser) Parse(html string) (*ParseResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	result := &ParseResult{
		Media: make([]MediaInfo, 0),
		Extra: make(map[string]interface{}),
	}

	// 获取标题
	title := doc.Find("article > div.post-content > h1").Text()
	if title != "" {
		result.Title = &title
	}

	urls, err := p.parseEpisodeVideo(html)
	if err != nil {
		return nil, fmt.Errorf("failed to parse episode video: %w", err)
	}

	for i, urlStr := range urls {
		videoURL, err := url.Parse(urlStr)
		if err != nil {
			continue // Skip invalid URLs
		}
		result.Media = append(result.Media, MediaInfo{
			URL:       videoURL,
			MediaType: Video,
			Filename:  p.getFileName(urlStr, i),
		})
	}

	return result, nil
}

func (p *DDYSParser) parseEpisodeVideo(html string) ([]string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	script := doc.Find(".wp-playlist-script").Text()
	if script == "" {
		return nil, fmt.Errorf("no playlist script found")
	}

	// 解析JSON数据
	type Track struct {
		Src0 string `json:"src0"`
	}
	type Playlist struct {
		Tracks []Track `json:"tracks"`
	}

	var playlist Playlist
	if err := json.Unmarshal([]byte(script), &playlist); err != nil {
		return nil, fmt.Errorf("failed to parse playlist JSON: %w", err)
	}

	// 构建视频URL列表
	urls := make([]string, 0, len(playlist.Tracks))
	for _, track := range playlist.Tracks {
		if track.Src0 != "" {
			urls = append(urls, "https://v.ddys.pro"+track.Src0)
		}
	}

	return urls, nil
}

// getFileName 生成文件名
func (p *DDYSParser) getFileName(urlPath string, index int) string {
	// 尝试从URL路径中提取文件名
	parts := strings.Split(urlPath, "/")
	if len(parts) > 0 {
		filename := parts[len(parts)-1]
		if filename != "" {
			return filename
		}
	}

	// 如果无法从URL中提取文件名，则生成一个序号文件名
	return fmt.Sprintf("video_%03d.mp4", index+1)
}

func (d *DDYSDownloader) Download(client *request.Client, url string, filepath string) error {
	opts := &request.RequestOption{
		Headers: map[string]string{
			"Accept":          "*/*",
			"Accept-Encoding": "identity;q=1, *;q=0",
			"Accept-Language": "en,zh-CN;q=0.9,zh;q=0.8",
			"Cache-Control":   "no-cache",
			"Connection":      "keep-alive",
			"Pragma":          "no-cache",
			"Sec-Fetch-Dest":  "video",
			"Sec-Fetch-Mode":  "no-cors",
			"Sec-Fetch-Site":  "cross-site",
			"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36",
			"Referer":         "https://ddys.pro/",
			"Origin":          "https://ddys.pro",
		},
	}
	return client.DownloadFile(url, filepath, opts)
}
