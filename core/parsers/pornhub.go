package parsers

import (
	"encoding/json"
	"fmt"
	"log"
	"media-crawler/core/request/client"
	"media-crawler/core/request/downloader"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type PornhubParser struct {
	client *client.Client
	DefaultDownloader
	prefix string // 保存从 master.m3u8 URL 中提取的前缀
}

func NewPornhubParser(client *client.Client) *PornhubParser {
	if client == nil {
		log.Printf("Warning: PornhubParser initialized with nil client")
	}
	return &PornhubParser{
		client: client,
	}
}

func (p *PornhubParser) GetDownloader() Downloader {
	return p
}

// Download 重写下载方法，使用存储的 prefix
func (p *PornhubParser) Download(client *client.Client, url string, filepath string) error {
	if p.prefix != "" {
		return p.DownloadWithPrefix(client, url, filepath, p.prefix)
	}
	return p.DefaultDownloader.Download(client, url, filepath)
}

// DownloadWithPrefix 带前缀的下载方法
func (p *PornhubParser) DownloadWithPrefix(client *client.Client, url string, filepath string, urlPrefix string) error {
	return downloader.NewDownloader(client, true).DownloadFileWithPrefix(url, filepath, nil, urlPrefix)
}

// GetPrefix 返回从 master.m3u8 URL 中提取的前缀
func (p *PornhubParser) GetPrefix() string {
	return p.prefix
}

type MediaDefinition struct {
	Quality  interface{} `json:"quality"` // 可能是字符串或数组
	VideoUrl string      `json:"videoUrl"`
	Remote   bool        `json:"remote"`
	Format   string      `json:"format"`
}

// getQualityString 从 quality 字段提取字符串值
func (m *MediaDefinition) getQualityString() string {
	switch v := m.Quality.(type) {
	case string:
		return v
	case []interface{}:
		// 如果是数组且为空，返回空字符串
		if len(v) == 0 {
			return ""
		}
		// 如果数组有元素，取第一个并转换为字符串
		if str, ok := v[0].(string); ok {
			return str
		}
		return ""
	default:
		return ""
	}
}

// writeDebugHTML 将 HTML 内容写入调试文件
func (p *PornhubParser) writeDebugHTML(html string, reason string) string {
	// 创建调试目录
	debugDir := "debug"
	os.MkdirAll(debugDir, 0755)

	// 生成唯一的文件名
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("pornhub_debug_%s_%s.html", timestamp, reason)
	filepath := filepath.Join(debugDir, filename)

	// 写入文件
	err := os.WriteFile(filepath, []byte(html), 0644)
	if err != nil {
		log.Printf("Failed to write debug HTML file: %v", err)
		return ""
	}

	return filepath
}

func (p *PornhubParser) Parse(html string) (*ParseResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		// 写入调试文件
		debugPath := p.writeDebugHTML(html, "parse_error")
		if debugPath != "" {
			log.Printf("Debug HTML saved to: %s", debugPath)
		}
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	result := &ParseResult{
		Media: make([]MediaInfo, 0),
		Extra: make(map[string]interface{}),
	}

	// 提取页面标题
	title := doc.Find("title").First().Text()
	if title != "" {
		result.Title = &title
	}

	// 从 script 标签中查找 mediaDefinitions
	var mediaDefinitions []MediaDefinition
	var foundMediaDefinitions bool

	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		scriptContent := s.Text()
		if strings.Contains(scriptContent, "mediaDefinitions") {
			log.Printf("Found script with mediaDefinitions, attempting to extract...")

			// 更精确的正则表达式来匹配 mediaDefinitions 数组
			// 匹配 "mediaDefinitions":[...] 的完整结构
			re := regexp.MustCompile(`"mediaDefinitions"\s*:\s*(\[(?:[^\[\]]|(?:\[[^\]]*\]))*\])`)
			matches := re.FindStringSubmatch(scriptContent)

			if len(matches) > 1 {
				jsonStr := matches[1]
				log.Printf("Extracted mediaDefinitions JSON: %s", jsonStr[:min(500, len(jsonStr))]+"...")

				err := json.Unmarshal([]byte(jsonStr), &mediaDefinitions)
				if err != nil {
					log.Printf("Warning: Failed to parse mediaDefinitions JSON: %v", err)

					// 尝试另一种匹配方式 - 匹配整个 flashvars 对象然后提取
					flashvarsRe := regexp.MustCompile(`var\s+flashvars_\d+\s*=\s*(\{.*?\});`)
					flashvarsMatches := flashvarsRe.FindStringSubmatch(scriptContent)
					if len(flashvarsMatches) > 1 {
						log.Printf("Trying to extract from flashvars object...")
						flashvarsStr := flashvarsMatches[1]

						// 从 flashvars 对象中提取 mediaDefinitions
						mediaDefRe := regexp.MustCompile(`"mediaDefinitions"\s*:\s*(\[(?:[^\[\]]|(?:\[[^\]]*\]))*\])`)
						mediaDefMatches := mediaDefRe.FindStringSubmatch(flashvarsStr)
						if len(mediaDefMatches) > 1 {
							jsonStr = mediaDefMatches[1]
							log.Printf("Re-extracted mediaDefinitions JSON from flashvars: %s", jsonStr[:min(100, len(jsonStr))]+"...")
							err = json.Unmarshal([]byte(jsonStr), &mediaDefinitions)
							if err == nil {
								foundMediaDefinitions = true
								log.Printf("Successfully parsed %d media definitions", len(mediaDefinitions))
							} else {
								log.Printf("Still failed to parse mediaDefinitions from flashvars: %v", err)
							}
						}
					}
				} else {
					foundMediaDefinitions = true
					log.Printf("Successfully parsed %d media definitions", len(mediaDefinitions))
				}
			} else {
				log.Printf("No matches found with mediaDefinitions regex")
			}
		}
	})

	if !foundMediaDefinitions || len(mediaDefinitions) == 0 {
		// 写入调试文件
		debugPath := p.writeDebugHTML(html, "no_media_definitions")
		if debugPath != "" {
			log.Printf("Debug HTML saved to: %s", debugPath)
		}
		return nil, fmt.Errorf("no mediaDefinitions found in page scripts")
	}

	// 选择质量最高的视频
	bestMedia := p.selectBestQuality(mediaDefinitions)
	if bestMedia == nil {
		// 写入调试文件并记录所有找到的媒体定义
		log.Printf("Found %d media definitions but none are valid:", len(mediaDefinitions))
		for i, media := range mediaDefinitions {
			qualityStr := media.getQualityString()
			log.Printf("  [%d] Quality: %s, VideoUrl: %s, Remote: %v", i, qualityStr,
				media.VideoUrl[:min(100, len(media.VideoUrl))], media.Remote)
		}

		debugPath := p.writeDebugHTML(html, "no_valid_media")
		if debugPath != "" {
			log.Printf("Debug HTML saved to: %s", debugPath)
		}
		return nil, fmt.Errorf("no valid media found in mediaDefinitions")
	}

	// 解析视频 URL
	videoURL, err := url.Parse(bestMedia.VideoUrl)
	if err != nil {
		// 写入调试文件
		debugPath := p.writeDebugHTML(html, "invalid_url")
		if debugPath != "" {
			log.Printf("Debug HTML saved to: %s", debugPath)
		}
		return nil, fmt.Errorf("failed to parse video URL: %w", err)
	}

	qualityStr := bestMedia.getQualityString()
	log.Printf("Selected best quality: %s, URL: %s", qualityStr, bestMedia.VideoUrl[:min(100, len(bestMedia.VideoUrl))])

	// 如果是 HLS master.m3u8，获取实际的分片 m3u8 链接
	finalVideoURL := videoURL
	if bestMedia.Format == "hls" && strings.Contains(bestMedia.VideoUrl, "master.m3u8") {
		log.Printf("Detected HLS master.m3u8, fetching segment m3u8...")
		finalVideoURL, err = url.Parse(bestMedia.VideoUrl)
		if err != nil {
			log.Printf("Warning: Failed to parse segment URL: %v, using master URL", err)
			finalVideoURL = videoURL
		} else {
			log.Printf("Successfully obtained segment m3u8 URL: %s", bestMedia.VideoUrl)
		}
	}

	// 添加到结果中
	filename := "video.mp4"
	if qualityStr != "" {
		filename = fmt.Sprintf("video_%s.mp4", qualityStr)
	}

	result.Media = append(result.Media, MediaInfo{
		URL:       finalVideoURL,
		MediaType: Video,
		Filename:  filename,
	})

	return result, nil
}

// selectBestQuality 从 mediaDefinitions 中选择质量最高的视频
func (p *PornhubParser) selectBestQuality(definitions []MediaDefinition) *MediaDefinition {
	var bestMedia *MediaDefinition
	maxQuality := 0

	for i := range definitions {
		media := &definitions[i]

		// 跳过远程媒体
		if media.Remote {
			continue
		}

		// 跳过空的视频 URL
		if media.VideoUrl == "" {
			continue
		}

		// 解析质量数值（如 "720" 从 "720p" 中提取）
		qualityNum := p.parseQualityNumber(media.Quality)

		if qualityNum > maxQuality {
			maxQuality = qualityNum
			bestMedia = media
		}
	}

	return bestMedia
}

// parseQualityNumber 从质量字段中提取数值
func (p *PornhubParser) parseQualityNumber(quality interface{}) int {
	switch v := quality.(type) {
	case string:
		re := regexp.MustCompile(`(\d+)`)
		matches := re.FindStringSubmatch(v)
		if len(matches) > 1 {
			if num, err := strconv.Atoi(matches[1]); err == nil {
				return num
			}
		}
	case []interface{}:
		if len(v) > 0 {
			if str, ok := v[0].(string); ok {
				re := regexp.MustCompile(`(\d+)`)
				matches := re.FindStringSubmatch(str)
				if len(matches) > 1 {
					if num, err := strconv.Atoi(matches[1]); err == nil {
						return num
					}
				}
			}
		}
	}
	return 0
}

// 添加辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
