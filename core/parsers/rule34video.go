package parsers

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"MediaNinja/core/request/client"
	"MediaNinja/utils/concurrent"

	"github.com/PuerkitoBio/goquery"
)

type Rule34VideoParser struct {
	options  ListOptions
	client   *client.Client
	entryURL *url.URL
	DefaultDownloader

	// downloadURL -> 视频页URL 的映射，用于下载直链签名过期（403）时重新换取新链接
	pageMu         sync.RWMutex
	pageByDownload map[string]string
}

func NewRule34VideoParser(c *client.Client, pageURL string) *Rule34VideoParser {
	entry, _ := url.Parse(pageURL)
	return &Rule34VideoParser{
		client:         c,
		entryURL:       entry,
		pageByDownload: make(map[string]string),
		options:        ListOptions{MaxPages: 1, Concurrency: 4},
	}
}

// SetListOptions 注入任务级列表参数（见 ListOptionsAware）。
func (p *Rule34VideoParser) SetListOptions(options ListOptions) { p.options = options }

type rule34VideoEntry struct {
	pageURL *url.URL
	title   string
	id      string
}

func (p *Rule34VideoParser) Parse(html string) (*ParseResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// 作者页/搜索页：div.thumbs 下有指向 /video/ 的缩略图链接
	if doc.Find("div.thumbs a.th[href*='/video/']").Length() > 0 {
		return p.parseListPage(doc)
	}
	return p.parseVideoPage(doc)
}

// parseVideoPage 解析单个视频页面，提取标题和最高清晰度的下载链接
func (p *Rule34VideoParser) parseVideoPage(doc *goquery.Document) (*ParseResult, error) {
	result := &ParseResult{
		Media: make([]MediaInfo, 0),
		Extra: make(map[string]interface{}),
	}

	title := strings.TrimSpace(doc.Find("title").Text())
	if title != "" {
		result.Title = &title
	}

	// 下载链接在下载面板内，可能包含多个清晰度（360p/480p/720p...）
	downloadPath := p.pickBestQuality(doc)
	if downloadPath == "" {
		// 回退到旧布局选择器：#tab_video_info > div:last-child > div 的第一个 a 标签
		fallback := doc.Find("#tab_video_info > div:last-child > div a").First()
		if fallback.Length() == 0 {
			return result, fmt.Errorf("download link not found")
		}
		downloadPath, _ = fallback.Attr("href")
	}
	if downloadPath == "" {
		return result, fmt.Errorf("download link href not found")
	}

	videoURL, err := p.normalizeURL(downloadPath)
	if err != nil {
		return result, fmt.Errorf("failed to normalize download URL: %w", err)
	}

	ext := path.Ext(strings.TrimSuffix(videoURL.Path, "/"))
	if ext == "" {
		ext = ".mp4"
	}

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

var rule34QualityRe = regexp.MustCompile(`_(\d{3,4})p?\.mp4`)

// pickBestQuality 从下载面板选择清晰度最高的 get_file 链接。
// 新版布局下载链接位于 div[data-sheet='download'] 面板，旧布局在 #tab_video_info 内
func (p *Rule34VideoParser) pickBestQuality(doc *goquery.Document) string {
	for _, sel := range []string{
		"div[data-sheet='download'] a[href*='/get_file/']",
		"#tab_video_info a[href*='/get_file/']",
		"a.tag_item_download[href*='/get_file/']",
	} {
		bestHref := ""
		bestRank := -1
		doc.Find(sel).Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if !exists || href == "" {
				return
			}
			rank := 0
			if m := rule34QualityRe.FindStringSubmatch(href); m != nil {
				if v, err := strconv.Atoi(m[1]); err == nil {
					rank = v
				}
			}
			if rank > bestRank {
				bestHref, bestRank = href, rank
			}
		})
		if bestHref != "" {
			return bestHref
		}
	}
	return ""
}

// parseListPage 解析作者页/搜索页：收集全部视频条目后，逐个抓取视频页汇总下载链接
func (p *Rule34VideoParser) parseListPage(doc *goquery.Document) (*ParseResult, error) {
	result := &ParseResult{
		Media: make([]MediaInfo, 0),
		Extra: make(map[string]interface{}),
	}

	listTitle := p.deriveListTitle()
	result.Title = &listTitle

	entries := extractVideoEntries(doc)

	// 翻页抓取（路径式分页：/models/xxx/2/）。搜索页的分页参数会被服务端忽略，
	// 直接构造页码 URL 时若返回空结果则停止翻页
	if p.client != nil {
		basePath := p.listBasePath()
		for page := 2; page <= p.options.MaxPages && basePath != ""; page++ {
			pageURL := "https://rule34video.com" + basePath + strconv.Itoa(page) + "/"
			sub, err := p.fetchDoc(pageURL)
			if err != nil {
				p.client.Logf("%s", fmt.Sprintf("Failed to fetch list page %s: %v", pageURL, err))
				result.Failures = append(result.Failures, err.Error())
				break
			}
			pageEntries := extractVideoEntries(sub)
			if len(pageEntries) == 0 {
				break
			}
			entries = appendEntries(entries, pageEntries)
		}
	}

	p.client.Logf("%s", fmt.Sprintf("Found %d videos on %s", len(entries), listTitle))

	if p.options.MaxVideos > 0 && len(entries) > p.options.MaxVideos {
		entries = entries[:p.options.MaxVideos]
		p.client.Logf("%s", fmt.Sprintf("Limited to first %d videos", p.options.MaxVideos))
	}

	// 并发抓取每个视频页，提取真实下载链接
	limiter := concurrent.NewLimiter(p.options.Concurrency)
	var mu sync.Mutex
	var firstErr error
	pageLinks := make([]string, 0, len(entries))

	for _, entry := range entries {
		entry := entry
		pageLinks = append(pageLinks, entry.pageURL.String())
		limiter.Execute(func() {
			media, err := p.resolveEntry(entry)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				result.Failures = append(result.Failures, err.Error())
				if firstErr == nil {
					firstErr = err
				}
				p.client.Logf("%s", fmt.Sprintf("Failed to resolve video %s: %v", entry.title, err))
				return
			}
			result.Media = append(result.Media, *media)
		})
	}
	limiter.Wait()

	result.Extra["video_pages"] = pageLinks

	if len(result.Media) == 0 && firstErr != nil {
		return result, firstErr
	}

	return result, nil
}

// resolveEntry 抓取单个视频页并生成 MediaInfo，文件名使用列表页标题 + 视频 ID 防止重名
func (p *Rule34VideoParser) resolveEntry(entry rule34VideoEntry) (*MediaInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("parser has no http client")
	}

	html, err := p.client.Get(entry.pageURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch video page: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("failed to parse video page: %w", err)
	}

	videoResult, err := p.parseVideoPage(doc)
	if err != nil {
		return nil, err
	}
	if len(videoResult.Media) == 0 {
		return nil, fmt.Errorf("no media parsed from video page")
	}

	media := videoResult.Media[0]
	p.pageMu.Lock()
	p.pageByDownload[media.URL.String()] = entry.pageURL.String()
	p.pageMu.Unlock()
	name := entry.title
	if name == "" && videoResult.Title != nil {
		name = *videoResult.Title
	}
	name = p.sanitizeFilename(name)
	if name == "" {
		name = "video"
	}
	if entry.id != "" {
		name = fmt.Sprintf("%s [%s]", name, entry.id)
	}
	ext := path.Ext(strings.TrimSuffix(media.URL.Path, "/"))
	if ext == "" {
		ext = ".mp4"
	}
	media.Filename = name + ext

	return &media, nil
}

// Download 下载视频；若直链返回 403（v-acctoken 签名过期），重抓视频页换取新链接后重试一次
func (p *Rule34VideoParser) Download(c *client.Client, rawURL string, filepath string) error {
	err := p.DefaultDownloader.Download(c, rawURL, filepath)
	if err == nil {
		return nil
	}
	if !strings.Contains(err.Error(), "403") {
		return err
	}

	p.pageMu.RLock()
	pageURL, ok := p.pageByDownload[rawURL]
	p.pageMu.RUnlock()
	if !ok {
		return err
	}

	p.client.Logf("%s", fmt.Sprintf("Download link expired (403), refreshing signed link from %s", pageURL))
	u, perr := url.Parse(pageURL)
	if perr != nil {
		return err
	}
	media, rerr := p.resolveEntry(rule34VideoEntry{pageURL: u})
	if rerr != nil || media == nil || media.URL.String() == rawURL {
		return err
	}
	return p.DefaultDownloader.Download(c, media.URL.String(), filepath)
}

// extractVideoEntries 从列表页 HTML 提取视频条目（页码 URL + 标题 + 视频 ID）
func extractVideoEntries(doc *goquery.Document) []rule34VideoEntry {
	seen := make(map[string]bool)
	entries := make([]rule34VideoEntry, 0)

	doc.Find("div.thumbs a.th[href*='/video/']").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}
		u, err := url.Parse(href)
		if err != nil {
			return
		}
		if seen[u.Path] {
			return
		}
		seen[u.Path] = true

		title := strings.TrimSpace(s.AttrOr("title", ""))
		if title == "" {
			title = strings.TrimSpace(s.Find("img").AttrOr("alt", ""))
		}

		entries = append(entries, rule34VideoEntry{
			pageURL: u,
			title:   title,
			id:      videoIDFromPath(u.Path),
		})
	})

	return entries
}

var rule34VideoIDRe = regexp.MustCompile(`/video/(\d+)/`)

func videoIDFromPath(p string) string {
	if m := rule34VideoIDRe.FindStringSubmatch(p); m != nil {
		return m[1]
	}
	return ""
}

// fetchDoc 抓取页面并解析为 goquery 文档
func (p *Rule34VideoParser) fetchDoc(pageURL string) (*goquery.Document, error) {
	html, err := p.client.Get(pageURL, nil)
	if err != nil {
		return nil, err
	}
	return goquery.NewDocumentFromReader(strings.NewReader(html))
}

var rule34PageNumRe = regexp.MustCompile(`/\d+/?$`)

// listBasePath 返回不含页码的列表页路径（以 / 结尾），例如 /models/juicyneko/
func (p *Rule34VideoParser) listBasePath() string {
	if p.entryURL == nil || p.entryURL.Path == "" {
		return ""
	}
	pt := p.entryURL.Path
	if !strings.Contains(pt, "/search/") && !strings.Contains(pt, "/models/") {
		return ""
	}
	pt = rule34PageNumRe.ReplaceAllString(pt, "/")
	if !strings.HasSuffix(pt, "/") {
		pt += "/"
	}
	return pt
}

// deriveListTitle 从入口 URL 推导目录名：/models/juicyneko/2/ -> juicyneko
func (p *Rule34VideoParser) deriveListTitle() string {
	if p.entryURL == nil {
		return "rule34_list"
	}
	segs := strings.Split(strings.Trim(p.entryURL.Path, "/"), "/")
	for i := len(segs) - 1; i >= 0; i-- {
		if segs[i] == "" {
			continue
		}
		// 跳过末尾的页码段
		if _, err := strconv.Atoi(segs[i]); err == nil {
			continue
		}
		return segs[i]
	}
	return "rule34_list"
}

// appendEntries 按 URL 路径去重合并条目
func appendEntries(dst, src []rule34VideoEntry) []rule34VideoEntry {
	seen := make(map[string]bool, len(dst))
	for _, e := range dst {
		seen[e.pageURL.Path] = true
	}
	for _, e := range src {
		if seen[e.pageURL.Path] {
			continue
		}
		seen[e.pageURL.Path] = true
		dst = append(dst, e)
	}
	return dst
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
		"<":  "_",
		">":  "_",
		":":  "_",
		"\"": "_",
		"/":  "_",
		"\\": "_",
		"|":  "_",
		"?":  "_",
		"*":  "_",
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
