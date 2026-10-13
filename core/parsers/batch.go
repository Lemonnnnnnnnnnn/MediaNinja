package parsers

import (
	"fmt"
	"net/url"
	"strings"
	"sync"

	"MediaNinja/core/request/client"
)

// ListOptions 控制列表页（搜索页/作者页）的枚举行为：最多抓取多少页、
// 保留多少条目、条目解析并发数。所有支持批次下载的解析器共用。
type ListOptions struct {
	MaxPages    int // 列表页最多抓取多少页（含第一页）
	MaxVideos   int // 每个列表页最多处理多少个条目，0 表示不限制
	Concurrency int // 抓取条目子页面的并发数
}

// ListOptionsAware 允许解析器接收任务级列表参数，工厂无需认识具体解析器类型。
type ListOptionsAware interface {
	SetListOptions(ListOptions)
}

// BatchSiteInfo 描述一个批次源，供能力发现（/api/session）使用。
type BatchSiteInfo struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Summary string `json:"summary"`
}

// BatchSource 把用户查询（创作者、关键词）解析为列表页 URL。
// 候选回退与探测期的错误处理属于站点语义，由实现自行决定。
type BatchSource interface {
	Info() BatchSiteInfo
	Resolve(c *client.Client, query string) (string, error)
}

var (
	batchMu      sync.RWMutex
	batchSources = map[string]BatchSource{}
	batchOrder   []string
)

// RegisterBatchSource 注册一个批次源；重复注册同一 ID 时覆盖实现、保持顺序。
func RegisterBatchSource(source BatchSource) {
	batchMu.Lock()
	defer batchMu.Unlock()
	info := source.Info()
	if info.ID == "" {
		panic("parsers: batch source requires an id")
	}
	if _, exists := batchSources[info.ID]; !exists {
		batchOrder = append(batchOrder, info.ID)
	}
	batchSources[info.ID] = source
}

// BatchSources 返回全部已注册批次源的描述，顺序即注册顺序。
func BatchSources() []BatchSiteInfo {
	batchMu.RLock()
	defer batchMu.RUnlock()
	out := make([]BatchSiteInfo, 0, len(batchOrder))
	for _, id := range batchOrder {
		out = append(out, batchSources[id].Info())
	}
	return out
}

// GetBatchSource 按 ID 查找批次源。
func GetBatchSource(id string) (BatchSource, error) {
	batchMu.RLock()
	defer batchMu.RUnlock()
	source, ok := batchSources[id]
	if !ok {
		return nil, fmt.Errorf("unknown batch source %q", id)
	}
	return source, nil
}

func init() {
	RegisterBatchSource(rule34VideoBatch{})
}

// rule34VideoBatch 优先解析创作者页，缺失时回退到关键词搜索。
type rule34VideoBatch struct{}

const rule34BaseURL = "https://rule34video.com"

func (rule34VideoBatch) Info() BatchSiteInfo {
	return BatchSiteInfo{
		ID:      "rule34video",
		Label:   "RULE34 VIDEO",
		Summary: "Creator pages are tried first. Missing creators fall back to keyword search.",
	}
}

func (rule34VideoBatch) Resolve(c *client.Client, query string) (string, error) {
	escaped := url.PathEscape(strings.TrimSpace(query))
	candidates := []string{
		rule34BaseURL + "/models/" + escaped + "/",
		rule34BaseURL + "/search/" + escaped + "/",
	}
	var lastStatus int
	for _, target := range candidates {
		response, err := c.GetStream("GET", target, nil, nil)
		// 探测期瞬时错误先重试一次再报告；只有 404 允许回退到下一候选。
		if err != nil {
			response, err = c.GetStream("GET", target, nil, nil)
		}
		if err != nil {
			return "", fmt.Errorf("%s: %w", query, err)
		}
		status := response.StatusCode
		response.Body.Close()
		if status == 200 {
			return target, nil
		}
		lastStatus = status
		if status != 404 {
			break
		}
	}
	if lastStatus == 404 {
		return "", fmt.Errorf("%s: no creator page or search results", query)
	}
	return "", fmt.Errorf("%s: list page HTTP status %d", query, lastStatus)
}
