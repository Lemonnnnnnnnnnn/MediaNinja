package parsers

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"media-crawler/core/request"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const tokenKey = "57A891D97E332A9D"

type NTDMParser struct {
	client *request.Client
	DefaultDownloader
}

func NewNTDMParser(client *request.Client) *NTDMParser {
	if client == nil {
		log.Printf("Warning: NTDMParser initialized with nil client")
	}
	return &NTDMParser{
		client: client,
	}
}

func (p *NTDMParser) Parse(html string) (*ParseResult, error) {
	title := p.parseTitle(html)
	log.Printf("Parsed title: %s", title)

	urls := p.parseEpisodeURLs(html)
	log.Printf("Found %d episode URLs", len(urls))

	result := &ParseResult{
		Media: make([]MediaInfo, 0),
		Extra: make(map[string]interface{}),
	}

	if title != "" {
		result.Title = &title
	}

	for i, episodeURL := range urls {
		log.Printf("Processing episode %d: %s", i+1, episodeURL)
		videoURL, err := p.parseEpisodeVideo(episodeURL)
		if err != nil {
			log.Printf("Failed to parse episode %d: %v", i+1, err)
			continue
		}
		if videoURL != "" {
			parsedURL, err := url.Parse(videoURL)
			if err != nil {
				log.Printf("Failed to parse URL %s: %v", videoURL, err)
				continue
			}
			result.Media = append(result.Media, MediaInfo{
				URL:       parsedURL,
				MediaType: Video,
				Filename:  fmt.Sprintf("%s-%d.mp4", title, i+1),
			})
			log.Printf("Successfully added video %d: %s", i+1, videoURL)
		}
	}

	log.Printf("Parsing completed, found %d videos", len(result.Media))
	return result, nil
}

func (p *NTDMParser) GetDownloader() Downloader {
	return p
}

func (p *NTDMParser) parseTitle(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}
	return doc.Find("#detailname > a").Text()
}

func (p *NTDMParser) parseEpisodeURLs(html string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}

	var urls []string
	doc.Find("#main0 > div:nth-child(1) > ul > li > a").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			urls = append(urls, "https://www.ntdm9.com"+href)
		}
	})
	return urls
}

func (p *NTDMParser) parseEpisodeVideo(url string) (string, error) {
	if p.client == nil {
		return "", fmt.Errorf("client is nil")
	}

	log.Printf("Fetching episode page: %s", url)
	html, err := p.client.GetHTML(url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch episode page: %w", err)
	}
	return p.parseVideoInfo(html)
}

func (p *NTDMParser) parseVideoInfo(html string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", err
	}

	videoScript := doc.Find("#ageframediv > script:nth-child(1)").Text()
	log.Printf("Found video script: %s", truncateString(videoScript, 100))

	re := regexp.MustCompile(`player_aaaa=(.*)`)
	matches := re.FindStringSubmatch(videoScript)
	if len(matches) < 2 {
		return "", fmt.Errorf("player_aaaa not found")
	}

	var playerInfo struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(matches[1]), &playerInfo); err != nil {
		return "", fmt.Errorf("failed to parse player info: %w", err)
	}

	if playerInfo.URL == "" {
		return "", fmt.Errorf("video URL not found")
	}

	yhdmURL := fmt.Sprintf("https://danmu.yhdmjx.com/m3u8.php?url=%s", playerInfo.URL)
	log.Printf("Generated YHDM URL: %s", yhdmURL)
	return p.parseYhdmURL(yhdmURL)
}

func (p *NTDMParser) parseYhdmURL(url string) (string, error) {
	log.Printf("Fetching YHDM page: %s", url)
	html, err := p.client.GetHTML(url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch yhdm page: %w", err)
	}

	btToken := p.parseBtToken(html)
	key := p.parseEncodedKey(html)
	log.Printf("Found btToken: %s, key length: %d", truncateString(btToken, 10), len(key))

	if btToken == "" || key == "" {
		return "", fmt.Errorf("missing decryption information")
	}

	decrypted, err := decryptVideoInfo(key, btToken)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt video info: %w", err)
	}
	log.Printf("Successfully decrypted video URL: %s", truncateString(decrypted, 50))
	return decrypted, nil
}

func (p *NTDMParser) parseBtToken(html string) string {
	re := regexp.MustCompile(`bt_token = "(.*)"`)
	matches := re.FindStringSubmatch(html)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func (p *NTDMParser) parseEncodedKey(html string) string {
	re := regexp.MustCompile(`"url": getVideoInfo\("(.*)"\)`)
	matches := re.FindStringSubmatch(html)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

// decryptVideoInfo decrypts the video information using AES decryption
func decryptVideoInfo(encryptedData, btToken string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 data: %w", err)
	}

	block, err := aes.NewCipher([]byte(tokenKey))
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	mode := cipher.NewCBCDecrypter(block, []byte(btToken))
	decrypted := make([]byte, len(ciphertext))
	mode.CryptBlocks(decrypted, ciphertext)

	paddingLen := int(decrypted[len(decrypted)-1])
	if paddingLen > len(decrypted) || paddingLen <= 0 {
		return "", fmt.Errorf("invalid padding length")
	}

	return string(decrypted[:len(decrypted)-paddingLen]), nil
}

// truncateString 用于截断长字符串，避免日志过长
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
