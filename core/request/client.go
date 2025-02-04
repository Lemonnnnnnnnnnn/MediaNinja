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
	"github.com/schollz/progressbar/v3"
	"golang.org/x/net/http2"
)

type RequestOption struct {
	Headers map[string]string
}

type Client struct {
	client         *http.Client
	proxy          string
	defaultHeaders map[string]string
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
		return u.tr1.RoundTrip(req)
	} else if req.URL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme: %s", req.URL.Scheme)
	}

	port := req.URL.Port()
	if port == "" {
		port = "443"
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%s", req.URL.Hostname(), port), 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("net.DialTimeout error: %+v", err)
	}

	uConn := tls.UClient(conn, &tls.Config{ServerName: req.URL.Hostname()}, tls.HelloCustom)
	if err = uConn.ApplyPreset(u.newSpec()); err != nil {
		return nil, fmt.Errorf("uConn.ApplyPreset() error: %+v", err)
	}
	if err = uConn.Handshake(); err != nil {
		return nil, fmt.Errorf("uConn.Handshake() error: %+v", err)
	}

	alpn := uConn.ConnectionState().NegotiatedProtocol
	switch alpn {
	case "h2":
		req.Proto = "HTTP/2.0"
		req.ProtoMajor = 2
		req.ProtoMinor = 0

		if c, err := u.tr2.NewClientConn(uConn); err == nil {
			return c.RoundTrip(req)
		} else {
			return nil, fmt.Errorf("http2.Transport.NewClientConn() error: %+v", err)
		}

	case "http/1.1", "":
		req.Proto = "HTTP/1.1"
		req.ProtoMajor = 1
		req.ProtoMinor = 1

		if err := req.Write(uConn); err == nil {
			return http.ReadResponse(bufio.NewReader(uConn), req)
		} else {
			return nil, fmt.Errorf("http.Request.Write() error: %+v", err)
		}

	default:
		return nil, fmt.Errorf("unsupported ALPN: %v", alpn)
	}
}

func NewClient(proxyURL string) *Client {
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
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	switch resp.StatusCode {
	case http.StatusOK:
		// 服务器不支持断点续传，需要重新下载
		startPos = 0
	case http.StatusPartialContent:
		// 服务器支持断点续传
	default:
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// 获取文件总大小
	contentLength := resp.ContentLength
	if contentLength <= 0 {
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

	// 创建进度条，降低刷新频率
	bar := progressbar.NewOptions64(
		totalSize,
		progressbar.OptionSetDescription("downloading"),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionThrottle(65*time.Millisecond), // 降低刷新频率
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(15),
		progressbar.OptionSetRenderBlankState(true),
	)

	// 设置断点续传的进度
	if startPos > 0 {
		bar.Add64(startPos)
	}

	// 使用缓冲读取提高性能
	bufSize := 32 * 1024 // 32KB buffer
	buf := make([]byte, bufSize)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			// 写入文件
			if _, werr := file.Write(buf[:n]); werr != nil {
				return fmt.Errorf("failed to write to file: %w", werr)
			}
			// 更新进度条
			bar.Add(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
	}

	return nil
}
