package types

import (
	"net/http"
	"time"
)

type Downloader interface {
	DownloadFile(url string, filepath string, opts *RequestOption) error
}

type ClientInterface interface {
	GetStream(method, url string, opts *RequestOption, headers map[string]string) (*http.Response, error)
	Get(url string, opts *RequestOption) (string, error)
	GetProxy() string
	GetMaxRetries() int
	GetRetryDelay() time.Duration
}
