package request

import (
	"testing"
)

func TestClientProxy(t *testing.T) {
	proxyURL := "http://127.0.0.1:7890"
	client := NewClient(proxyURL, 3, 5)

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "HTTP URL",
			url:     "http://example.com",
			wantErr: false,
		},
		{
			name:    "HTTPS URL",
			url:     "https://example.com",
			wantErr: false,
		},
		{
			name:    "GOOGLE URL",
			url:     "https://www.google.com",
			wantErr: false,
		},
		{
			name:    "Baidu URL",
			url:     "https://baidu.com",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetHTML(tt.url, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetHTML() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
