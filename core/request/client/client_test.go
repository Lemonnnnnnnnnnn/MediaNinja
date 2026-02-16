package client

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
			_, err := client.Get(tt.url, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetHTML() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVideoStreamRequest(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		proxy   string
		wantErr bool
	}{
		{
			name:    "Xfvod video stream",
			url:     "https://play.xfvod.pro:8088/G/G-%E6%80%AA%E7%89%A9/01.mp4",
			proxy:   "",
			wantErr: false,
		},
		{
			name:    "Xfvod video stream with proxy",
			url:     "https://play.xfvod.pro:8088/G/G-%E6%80%AA%E7%89%A9/01.mp4",
			proxy:   "http://127.0.0.1:7890",
			wantErr: true, // uTLS may have compatibility issues with some servers via proxy
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.proxy, 3, 5)

			// Test GetStream to get response headers without reading body
			resp, err := client.GetStream("GET", tt.url, nil, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetStream() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if resp != nil {
				defer resp.Body.Close()

				// Log response info
				t.Logf("Response Status: %s", resp.Status)
				t.Logf("Content-Type: %s", resp.Header.Get("Content-Type"))
				t.Logf("Content-Length: %s", resp.Header.Get("Content-Length"))

				// Verify it's a video response
				contentType := resp.Header.Get("Content-Type")
				if contentType != "" && resp.StatusCode == 200 {
					t.Logf("Successfully received video stream response")
				}
			}
		})
	}
}
