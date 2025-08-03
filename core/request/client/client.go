package client

import (
	"fmt"
	"io"
	"MediaNinja/core/request/types"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/http2"
)

type RequestOption = types.RequestOption
type DownloadOption = types.DownloadOption
type Downloader = types.Downloader

type Client struct {
	Client         *http.Client
	Proxy          string
	DefaultHeaders map[string]string
	MaxRetries     int
	RetryDelay     time.Duration
}

func NewClient(proxyURL string, maxRetries int, retryDelay int) *Client {
	transport := &uTransport{
		tr1: &http.Transport{},
		tr2: &http2.Transport{},
	}

	if proxyURL != "" {
		proxyUrl, _ := url.Parse(proxyURL)
		transport.tr1.Proxy = http.ProxyURL(proxyUrl)
	}

	return &Client{
		Client: &http.Client{Transport: transport},
		Proxy:  proxyURL,
		DefaultHeaders: map[string]string{
			"accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
			"accept-language": "en,zh-CN;q=0.9,zh;q=0.8",
			"user-agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36",
		},
		MaxRetries: maxRetries,
		RetryDelay: time.Duration(retryDelay) * time.Second,
	}

}

func (c *Client) setHeaders(req *http.Request, opts *RequestOption) {
	// 如果有临时请求头，只使用临时请求头
	if opts != nil && opts.Headers != nil {
		for k, v := range opts.Headers {
			req.Header.Set(k, v)
		}
		return
	}

	// 如果没有临时请求头，使用默认请求头
	for k, v := range c.DefaultHeaders {
		req.Header.Set(k, v)
	}

}

// GetStream 执行请求并返回响应流，需要调用者负责关闭响应体
func (c *Client) GetStream(method, url string, opts *RequestOption, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req, opts)

	// 设置额外的请求头
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return resp, nil
}

// Get 方法重构
func (c *Client) Get(url string, opts *RequestOption) (string, error) {
	resp, err := c.GetStream("GET", url, opts, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (c *Client) GetProxy() string {
	return c.Proxy
}

func (c *Client) GetMaxRetries() int {
	return c.MaxRetries
}

func (c *Client) GetRetryDelay() time.Duration {
	return c.RetryDelay
}
