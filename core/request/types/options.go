package types

// RequestOption 定义请求选项
type RequestOption struct {
	Headers map[string]string
}

// DownloadOption 定义下载选项
type DownloadOption struct {
	MaxRetries  int
	RetryDelay  int
	MaxParallel int
}
