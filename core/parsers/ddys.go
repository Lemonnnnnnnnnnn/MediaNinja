package parsers

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"media-crawler/core/request/client"
	"media-crawler/core/request/downloader"
	"media-crawler/core/request/types"
	"media-crawler/utils/format"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type DDYSParser struct {
	client     *client.Client
	downloader DDYSDownloader
}

func NewDDYSParser(client *client.Client) *DDYSParser {
	if client == nil {
		log.Printf("Warning: NTDMParser initialized with nil client")
	}
	return &DDYSParser{
		client: client,
	}
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

	urls, subtitleURLs, err := p.parseEpisodeVideo(html)
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
			Filename:  format.GetFileName(urlStr, i),
		})
	}

	for _, subtitleURLStr := range subtitleURLs {
		data, err := p.fetchSubtitleThenDecrypt(subtitleURLStr)
		if err != nil {
			log.Printf("Failed to process subtitle: %v", err)
			continue
		}

		result.Files = append(result.Files, FileContent{
			Filename:    formatSubtitlePath(subtitleURLStr),
			ContentType: "text",
			Data:        data,
		})
	}

	return result, nil
}

func (p *DDYSParser) parseEpisodeVideo(html string) ([]string, []string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse HTML: %w", err)

	}

	script := doc.Find(".wp-playlist-script").Text()
	if script == "" {
		return nil, nil, fmt.Errorf("no playlist script found")
	}

	// 解析JSON数据
	type Track struct {
		Src0   string `json:"src0"`
		SubSrc string `json:"subsrc"`
	}
	type Playlist struct {
		Tracks []Track `json:"tracks"`
	}

	var playlist Playlist
	if err := json.Unmarshal([]byte(script), &playlist); err != nil {
		return nil, nil, fmt.Errorf("failed to parse playlist JSON: %w", err)
	}

	// 构建视频URL列表
	urls := make([]string, 0, len(playlist.Tracks))
	subtitleURLs := make([]string, 0, len(playlist.Tracks))
	for _, track := range playlist.Tracks {
		if track.Src0 != "" {
			urls = append(urls, "https://v.ddys.pro"+track.Src0)
		}
		if track.SubSrc != "" {
			subtitleURLs = append(subtitleURLs, "https://ddys.pro/subddr"+track.SubSrc)
		}
	}

	return urls, subtitleURLs, nil
}

func (d *DDYSDownloader) Download(client *client.Client, url string, filepath string) error {
	opts := &types.RequestOption{
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
	return downloader.NewDownloader(client, true).DownloadFile(url, filepath, opts)
}

func (d *DDYSDownloader) DownloadWithPrefix(client *client.Client, url string, filepath string, urlPrefix string) error {
	return nil
}

func (p *DDYSParser) fetchSubtitleThenDecrypt(url string) (string, error) {
	resp, err := p.client.GetStream("GET", url, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch subtitle: %w", err)
	}
	defer resp.Body.Close()

	encodedSubtitleFile, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read subtitle: %w", err)
	}

	data, err := DecryptSubtitle(encodedSubtitleFile)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt subtitle: %w", err)
	}

	return data, nil
}

func formatSubtitlePath(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return "subtitle.vtt"
	}

	filename := parts[len(parts)-1]
	if dotIndex := strings.LastIndex(filename, "."); dotIndex != -1 {
		filename = filename[:dotIndex]
	}

	return filename + ".vtt"
}

func DecryptSubtitle(encryptedData []byte) (string, error) {
	if len(encryptedData) < 16 {
		return "", fmt.Errorf("invalid data: too short (length: %d)", len(encryptedData))
	}

	// Split IV and ciphertext
	iv := encryptedData[:16]
	ciphertext := encryptedData[16:]

	// Decrypt
	plaintext, err := decryptAES_CBC(ciphertext, iv)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	// Gzip decompress
	subtitleContent, err := ungzipData(plaintext)
	if err != nil {
		return "", fmt.Errorf("gzip decompression failed: %w", err)
	}

	// 标准化处理：删除所有 "&lrm;" 字符串
	normalizedContent := strings.ReplaceAll(subtitleContent, "&lrm;", "")

	return normalizedContent, nil
}

func decryptAES_CBC(ciphertext, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(iv)
	if err != nil {
		return nil, err

	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	return pkcs7Unpad(plaintext, aes.BlockSize)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, fmt.Errorf("invalid padding size")
	}

	// 获取填充字节数
	padding := int(data[length-1])
	if padding < 1 || padding > blockSize {
		return nil, fmt.Errorf("invalid padding")
	}

	return data[:length-padding], nil
}

func ungzipData(data []byte) (string, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var result bytes.Buffer
	_, err = io.Copy(&result, reader)
	if err != nil {
		return "", err
	}

	return result.String(), nil
}
