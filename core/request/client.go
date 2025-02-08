package request

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	tls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
)

type RequestOption struct {
	Headers map[string]string
}

type Client struct {
	client         *http.Client
	proxy          string
	defaultHeaders map[string]string
	maxRetries     int
	retryDelay     time.Duration
}

type uTransport struct {
	tr1 *http.Transport
	tr2 *http2.Transport
}

func (*uTransport) newSpec() *tls.ClientHelloSpec {
	return &tls.ClientHelloSpec{
		TLSVersMax:         tls.VersionTLS13,
		TLSVersMin:         tls.VersionTLS12,
		CipherSuites:       []uint16{tls.GREASE_PLACEHOLDER, 0x1301, 0x1302, 0x1303, 0xc02b, 0xc02f, 0xc02c, 0xc030, 0xcca9, 0xcca8, 0xc013, 0xc014, 0x009c, 0x009d, 0x002f, 0x0035},
		CompressionMethods: []uint8{0x0},
		Extensions: []tls.TLSExtension{
			&tls.UtlsGREASEExtension{},
			&tls.SNIExtension{},
			&tls.ExtendedMasterSecretExtension{},
			&tls.RenegotiationInfoExtension{},
			&tls.SupportedCurvesExtension{Curves: []tls.CurveID{tls.GREASE_PLACEHOLDER, tls.X25519, tls.CurveP256, tls.CurveP384}},
			&tls.SupportedPointsExtension{SupportedPoints: []byte{0x0}},
			&tls.SessionTicketExtension{},
			&tls.ALPNExtension{AlpnProtocols: []string{"http/1.1"}},
			&tls.StatusRequestExtension{},
			&tls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []tls.SignatureScheme{0x0403, 0x0804, 0x0401, 0x0503, 0x0805, 0x0501, 0x0806, 0x0601}},
			&tls.SCTExtension{},
			&tls.KeyShareExtension{KeyShares: []tls.KeyShare{
				{Group: tls.CurveID(tls.GREASE_PLACEHOLDER), Data: []byte{0}},
				{Group: tls.X25519},
			}},
			&tls.PSKKeyExchangeModesExtension{Modes: []uint8{tls.PskModeDHE}},
			&tls.SupportedVersionsExtension{Versions: []uint16{tls.GREASE_PLACEHOLDER, tls.VersionTLS13, tls.VersionTLS12}},
			&tls.UtlsCompressCertExtension{Algorithms: []tls.CertCompressionAlgo{tls.CertCompressionBrotli}},
			&tls.ApplicationSettingsExtension{SupportedProtocols: []string{"h2"}},
			&tls.UtlsGREASEExtension{},
			&tls.UtlsPaddingExtension{GetPaddingLen: tls.BoringPaddingStyle},
		},
		GetSessionID: nil,
	}
}

func (u *uTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme == "http" {
		// 对于 http 请求，直接使用普通 RoundTrip 处理
		return u.tr1.RoundTrip(req)
	} else if req.URL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme: %s", req.URL.Scheme)
	}

	// 从 transport 配置中获取代理地址
	proxyURL, err := u.tr1.Proxy(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get proxy URL: %v", err)
	}
	if proxyURL == nil {
		return nil, fmt.Errorf("proxy URL is not configured")
	}

	// 连接到 HTTP 代理
	conn, err := net.DialTimeout("tcp", proxyURL.Host, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %v", err)
	}

	// 发送 CONNECT 请求
	dest := req.URL.Host
	if req.URL.Port() == "" {
		dest += ":443"
	}
	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", dest, req.URL.Host)
	_, err = conn.Write([]byte(connectReq))
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send CONNECT request: %v", err)
	}

	// 读取代理响应
	respReader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(respReader, req)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read proxy response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("proxy CONNECT failed with status: %s", resp.Status)
	}

	// 代理建立了隧道，创建 TLS 连接
	tlsConn := tls.UClient(conn, &tls.Config{ServerName: req.URL.Hostname()}, tls.HelloCustom)
	if err = tlsConn.ApplyPreset(u.newSpec()); err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("uConn.ApplyPreset() error: %+v", err)
	}
	if err = tlsConn.Handshake(); err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %+v", err)
	}

	// 处理 ALPN (HTTP/2 或 HTTP/1.1)
	alpn := tlsConn.ConnectionState().NegotiatedProtocol
	switch alpn {
	case "h2":
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0

		if c, err := u.tr2.NewClientConn(tlsConn); err == nil {
			return c.RoundTrip(req)
		} else {
			return nil, fmt.Errorf("http2.Transport.NewClientConn() error: %+v", err)
		}

	case "http/1.1", "":
		req.Proto = "HTTP/1.1"
		req.ProtoMajor = 1
		req.ProtoMinor = 1

		if err := req.Write(tlsConn); err == nil {
			return http.ReadResponse(bufio.NewReader(tlsConn), req)
		} else {
			return nil, fmt.Errorf("http.Request.Write() error: %+v", err)
		}

	default:
		return nil, fmt.Errorf("unsupported ALPN: %v", alpn)
	}
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
		client: &http.Client{Transport: transport},
		proxy:  proxyURL,
		defaultHeaders: map[string]string{
			"accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
			"accept-language": "en,zh-CN;q=0.9,zh;q=0.8",
			"user-agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36",
		},
		maxRetries: maxRetries,
		retryDelay: time.Duration(retryDelay) * time.Second,
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
	for k, v := range c.defaultHeaders {
		req.Header.Set(k, v)
	}
}

func (c *Client) GetHTML(url string, opts *RequestOption) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	c.setHeaders(req, opts)

	resp, err := c.client.Do(req)
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

func (c *Client) DownloadFile(url string, filepath string, opts *RequestOption) error {
	if err := os.MkdirAll(path.Dir(filepath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	var attempt int
	for attempt = 0; attempt < c.maxRetries; attempt++ {
		if attempt > 0 {
			// 重试前等待一段时间
			time.Sleep(c.retryDelay)
		}

		// 获取文件信息用于断点续传
		var startPos int64 = 0
		fi, err := os.Stat(filepath)
		if err == nil {
			startPos = fi.Size()
		}

		// 创建请求
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		c.setHeaders(req, opts)

		// 设置断点续传
		if startPos > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startPos))
		}

		// 发送请求
		resp, err := c.client.Do(req)
		if err != nil {
			if attempt < c.maxRetries-1 {
				fmt.Printf("Download attempt %d failed: %v, retrying...\n", attempt+1, err)
				continue
			}
			return fmt.Errorf("failed to send request after %d attempts: %w", c.maxRetries, err)
		}
		defer resp.Body.Close()

		// 检查响应状态
		switch resp.StatusCode {
		case http.StatusOK:
			startPos = 0
		case http.StatusPartialContent:
			// 服务器支持断点续传
		case http.StatusRequestedRangeNotSatisfiable: // 416 错误码
			// 文件已经完全下载，直接返回成功
			return nil
		default:
			if attempt < c.maxRetries-1 {
				fmt.Printf("Download attempt %d failed with status code %d, retrying...\n", attempt+1, resp.StatusCode)
				continue
			}
			return fmt.Errorf("unexpected status code after %d attempts: %d", c.maxRetries, resp.StatusCode)
		}

		// 获取文件总大小
		contentLength := resp.ContentLength
		if contentLength <= 0 {
			if attempt < c.maxRetries-1 {
				fmt.Printf("Download attempt %d failed: invalid content length, retrying...\n", attempt+1)
				continue
			}
			return fmt.Errorf("invalid content length: %d", contentLength)
		}
		totalSize := startPos + contentLength

		// 打开或创建文件
		flags := os.O_CREATE | os.O_WRONLY
		if startPos > 0 {
			flags |= os.O_APPEND
		}
		file, err := os.OpenFile(filepath, flags, 0644)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		// 创建进度跟踪器
		progress := NewDownloadProgress(path.Base(filepath), totalSize, startPos)

		// 使用缓冲读取提高性能
		bufSize := 32 * 1024 // 32KB buffer
		buf := make([]byte, bufSize)

		downloadSuccess := true
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				// 写入文件
				if _, werr := file.Write(buf[:n]); werr != nil {
					progress.Fail(werr)
					downloadSuccess = false
					if attempt < c.maxRetries-1 {
						fmt.Printf("Download attempt %d failed while writing: %v, retrying...\n", attempt+1, werr)
						break
					}
					return fmt.Errorf("failed to write to file: %w", werr)
				}
				progress.Update(int64(n))
			}
			if err == io.EOF {
				progress.Success()
				return nil // 下载成功，直接返回
			}
			if err != nil {
				progress.Fail(err)
				downloadSuccess = false
				if attempt < c.maxRetries-1 {
					fmt.Printf("Download attempt %d failed while reading: %v, retrying...\n", attempt+1, err)
					break
				}
				return fmt.Errorf("failed to read response: %w", err)
			}
		}

		if downloadSuccess {
			return nil
		}
	}

	return fmt.Errorf("download failed after %d attempts", c.maxRetries)
}
