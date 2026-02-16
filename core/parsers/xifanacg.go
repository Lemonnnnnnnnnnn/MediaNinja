package parsers

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"MediaNinja/core/request/client"
	"MediaNinja/core/request/downloader"
	"MediaNinja/core/request/types"
	"MediaNinja/utils/concurrent"
	"net/url"

	"github.com/PuerkitoBio/goquery"
)

type XifanacgParser struct {
	client     *client.Client
	downloader XifanacgDownloader
	limiter    *concurrent.Limiter
}

type XifanacgDownloader struct{}

func NewXifanacgParser(client *client.Client) *XifanacgParser {
	if client == nil {
		log.Printf("Warning: XifanacgParser initialized with nil client")
	}
	return &XifanacgParser{
		client:     client,
		downloader: XifanacgDownloader{},
		limiter:    concurrent.NewLimiter(3), // Fixed limit of 3
	}
}

func (p *XifanacgParser) Parse(html string) (*ParseResult, error) {
	title := p.parseTitle(html)
	log.Printf("Parsed title: %s", title)

	episodeURLs := p.parseEpisodeURLs(html)
	log.Printf("Found %d episode URLs", len(episodeURLs))

	// Extract current episode video URL from the input HTML
	currentVideoURL, err := p.parseVideoURLFromHTML(html)
	if err != nil {
		log.Printf("Failed to parse current episode video: %v", err)
	} else {
		log.Printf("Found current episode video: %s", currentVideoURL)
	}

	result := &ParseResult{
		Media: make([]MediaInfo, 0),
		Extra: make(map[string]interface{}),
	}

	if title != "" {
		result.Title = &title
	}

	// Add current episode video if found
	if currentVideoURL != "" {
		parsedURL, err := url.Parse(currentVideoURL)
		if err == nil {
			// Try to extract episode number from current URL
			episodeNum := p.extractEpisodeNumber(currentVideoURL)
			filename := fmt.Sprintf("%s-%s.mp4", title, episodeNum)

			result.Media = append(result.Media, MediaInfo{
				URL:       parsedURL,
				MediaType: Video,
				Filename:  filename,
			})
			log.Printf("Added current episode video: %s", filename)
		}
	}

	// Process other episodes concurrently with limiter
	type episodeResult struct {
		index     int
		mediaInfo *MediaInfo
		err       error
	}
	resultChan := make(chan episodeResult, len(episodeURLs))

	for i, episodeURL := range episodeURLs {
		idx, u := i, episodeURL
		p.limiter.Execute(func() {
			log.Printf("Processing episode %d: %s", idx+1, u)
			videoURL, err := p.parseEpisodeVideoURL(u)
			if err != nil {
				resultChan <- episodeResult{idx, nil, err}
				return
			}

			if videoURL == "" {
				resultChan <- episodeResult{idx, nil, nil}
				return
			}

			parsedURL, err := url.Parse(videoURL)
			if err != nil {
				resultChan <- episodeResult{idx, nil, err}
				return
			}

			// Extract episode number from video URL
			episodeNum := p.extractEpisodeNumber(videoURL)
			filename := fmt.Sprintf("%s-%s.mp4", title, episodeNum)

			mediaInfo := &MediaInfo{
				URL:       parsedURL,
				MediaType: Video,
				Filename:  filename,
			}
			resultChan <- episodeResult{idx, mediaInfo, nil}
		})
	}

	p.limiter.Wait()
	close(resultChan)

	// Collect all results
	mediaInfos := make([]*MediaInfo, len(episodeURLs))
	for range episodeURLs {
		res := <-resultChan
		if res.err != nil {
			log.Printf("Failed to parse episode %d: %v", res.index+1, res.err)
			continue
		}
		if res.mediaInfo != nil {
			mediaInfos[res.index] = res.mediaInfo
			log.Printf("Successfully added video %d: %s", res.index+1, res.mediaInfo.URL.String())
		}
	}

	// Add all non-empty media info to result
	for _, info := range mediaInfos {
		if info != nil {
			result.Media = append(result.Media, *info)
		}
	}

	log.Printf("Parsing completed, found %d videos", len(result.Media))
	return result, nil
}

func (p *XifanacgParser) GetDownloader() Downloader {
	return &p.downloader
}

func (d *XifanacgDownloader) Download(_client *client.Client, url string, filepath string) error {
	client := client.NewClient("", 3, 1)
	opts := &types.RequestOption{
		Headers: map[string]string{
			"Accept":          "*/*",
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
			"Referer":         "https://dm.xifanacg.com/",
			"Origin":          "https://dm.xifanacg.com",
			"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Sec-Fetch-Dest":  "video",
			"Sec-Fetch-Mode":  "cors",
			"Sec-Fetch-Site":  "cross-site",
		},
	}
	return downloader.NewDownloader(client, true).DownloadFile(url, filepath, opts)
}

func (d *XifanacgDownloader) DownloadWithPrefix(client *client.Client, url string, filepath string, urlPrefix string) error {
	return nil
}

func (p *XifanacgParser) parseTitle(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}

	// Try to find title in h2.player-title-link first
	if title := doc.Find("h2.player-title-link").Text(); title != "" {
		return strings.TrimSpace(title)
	}

	// Fallback to title tag
	if title := doc.Find("title").Text(); title != "" {
		// Clean title - remove episode number and suffixes
		title = strings.TrimSpace(title)
		// Remove episode number like "_第01集"
		if idx := strings.Index(title, "_第"); idx > 0 {
			title = title[:idx]
		}
		// Remove common suffixes
		title = strings.TrimSuffix(title, "_完结旧番 - 稀饭动漫 - 免费高清动漫分享")
		title = strings.TrimSuffix(title, " - 戏枫动漫")
		title = strings.TrimSuffix(title, " - Xifanacg")
		title = strings.TrimSuffix(title, " - 稀饭动漫")
		return strings.TrimSpace(title)
	}

	return ""
}

func (p *XifanacgParser) parseEpisodeURLs(html string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}

	var urls []string
	doc.Find("ul.anthology-list-play.size > li > a").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			// Skip if it's a relative URL that's empty or just a fragment
			if href != "" && !strings.HasPrefix(href, "#") {
				// Build absolute URL
				if strings.HasPrefix(href, "/") {
					urls = append(urls, "https://dm.xifanacg.com"+href)
				} else if strings.HasPrefix(href, "http") {
					urls = append(urls, href)
				}
			}
		}
	})

	return urls
}

func (p *XifanacgParser) parseVideoURLFromHTML(html string) (string, error) {
	// Extract player_aaaa variable using regex
	// The actual format: player_aaaa={...}</script> (no 'var' keyword, no semicolon)
	// Need to match the entire JSON object including nested objects
	patterns := []string{
		`player_aaaa=(\{.*?\})</script>`, // Match until </script> - non-greedy
		`var player_aaaa=(\{.*?\});`,
		`var player_aaaa=(\{.*?\})`,
		`player_aaaa\s*=\s*(\{.*?\});`,
		`player_aaaa\s*=\s*(\{.*?\})`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) >= 2 {
			var playerInfo struct {
				URL string `json:"url"`
			}
			if err := json.Unmarshal([]byte(matches[1]), &playerInfo); err != nil {
				log.Printf("Failed to parse JSON with pattern '%s': %v", pattern, err)
				continue // Try next pattern
			}

			if playerInfo.URL != "" {
				return playerInfo.URL, nil
			}
		}
	}

	return "", fmt.Errorf("player_aaaa variable not found in any expected pattern")
}

func (p *XifanacgParser) parseEpisodeVideoURL(episodeURL string) (string, error) {
	if p.client == nil {
		return "", fmt.Errorf("client is nil")
	}

	log.Printf("Fetching episode page: %s", episodeURL)
	html, err := p.client.Get(episodeURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch episode page: %w", err)
	}

	return p.parseVideoURLFromHTML(html)
}

func (p *XifanacgParser) extractEpisodeNumber(videoURL string) string {
	// Try to extract episode number from video URL path
	// Expected format: https://play.xfvod.pro:8088/G/G-怪物/01.mp4
	parts := strings.Split(videoURL, "/")
	if len(parts) > 0 {
		filename := parts[len(parts)-1]
		// Remove extension
		if idx := strings.LastIndex(filename, "."); idx > 0 {
			filename = filename[:idx]
		}
		return filename
	}

	// Fallback: try to extract from path
	for _, part := range parts {
		if strings.Contains(part, ".mp4") {
			return strings.TrimSuffix(part, ".mp4")
		}
	}

	// Last resort: return a counter or placeholder
	return "episode"
}
