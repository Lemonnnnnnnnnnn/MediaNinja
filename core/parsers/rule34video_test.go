package parsers

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestRule34ParseVideoPage(t *testing.T) {
	tests := []struct {
		name         string
		html         string
		wantTitle    string
		wantURLPart  string
		wantFilename string
		wantErr      bool
	}{
		{
			name: "新布局：下载面板中选择最高清晰度",
			html: `<html><title>Test Video [X]</title><body>
				<div id="tab_video_info">
					<div class="row"><div><a href="/tags/1/">tag</a></div></div>
				</div>
				<div class="vp-sheet" data-sheet="download" hidden>
					<div class="fs-sheet__body vp-sheet__body">
						<a class="tag_item_download" href="https://rule34video.com/get_file/1/aaa/3318139_360p.mp4/?r=1&download=true">360p</a>
						<a class="tag_item_download" href="https://rule34video.com/get_file/1/bbb/3318139_720p.mp4/?r=1&download=true">720p</a>
						<a class="tag_item_download" href="https://rule34video.com/get_file/1/ccc/3318139_480p.mp4/?r=1&download=true">480p</a>
					</div>
				</div>
			</body></html>`,
			wantTitle:    "Test Video [X]",
			wantURLPart:  "3318139_720p.mp4",
			wantFilename: "Test Video [X].mp4",
		},
		{
			name: "旧布局：tab_video_info 内选择最高清晰度",
			html: `<html><title>Legacy Page</title><body>
				<div id="tab_video_info">
					<div class="row"><div><a href="/tags/1/">tag</a></div></div>
					<div class="row downloads"><div>
						<a href="https://rule34video.com/get_file/1/aaa/3318139_360p.mp4/?r=1">360p</a>
						<a href="https://rule34video.com/get_file/1/bbb/3318139_720p.mp4/?r=1">720p</a>
					</div></div>
				</div>
			</body></html>`,
			wantTitle:    "Legacy Page",
			wantURLPart:  "3318139_720p.mp4",
			wantFilename: "Legacy Page.mp4",
		},
		{
			name: "无清晰度后缀时回退第一个链接",
			html: `<html><title>Fallback</title><body>
				<div id="tab_video_info">
					<div class="row downloads"><div>
						<a href="https://rule34video.com/get_file/1/ddd/3318139.mp4/">download</a>
					</div></div>
				</div>
			</body></html>`,
			wantTitle:    "Fallback",
			wantURLPart:  "3318139.mp4",
			wantFilename: "Fallback.mp4",
		},
		{
			name: "没有下载链接时报错",
			html: `<html><title>Empty</title><body><div id="tab_video_info"></div></body></html>`,
			wantErr: true,
		},
	}

	p := &Rule34VideoParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Parse(tt.html)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Title == nil || *result.Title != tt.wantTitle {
				t.Errorf("title = %v, want %s", result.Title, tt.wantTitle)
			}
			if len(result.Media) != 1 {
				t.Fatalf("media count = %d, want 1", len(result.Media))
			}
			m := result.Media[0]
			if !strings.Contains(m.URL.String(), tt.wantURLPart) {
				t.Errorf("url = %s, want contains %s", m.URL.String(), tt.wantURLPart)
			}
			if m.Filename != tt.wantFilename {
				t.Errorf("filename = %s, want %s", m.Filename, tt.wantFilename)
			}
		})
	}
}

func TestRule34ExtractVideoEntries(t *testing.T) {
	html := `<html><body>
		<div class="thumbs clearfix">
			<div class="item thumb">
				<a class="th" href="https://rule34video.com/video/111/aaa/" title="Video A"><img src="x.jpg" alt="Video A"/></a>
			</div>
			<div class="item thumb">
				<a class="th" href="https://rule34video.com/video/222/bbb/" title="Video B"></a>
			</div>
			<div class="item thumb">
				<a class="th" href="https://rule34video.com/video/111/aaa/" title="Duplicate"></a>
			</div>
		</div>
	</body></html>`

	doc := mustDoc(t, html)
	entries := extractVideoEntries(doc)
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].id != "111" || entries[0].title != "Video A" {
		t.Errorf("entry[0] = %+v", entries[0])
	}
	if entries[1].id != "222" || entries[1].title != "Video B" {
		t.Errorf("entry[1] = %+v", entries[1])
	}
}

func TestRule34ListPaths(t *testing.T) {
	tests := []struct {
		entry        string
		wantBasePath string
		wantTitle    string
	}{
		{"https://rule34video.com/models/juicyneko/", "/models/juicyneko/", "juicyneko"},
		{"https://rule34video.com/models/juicyneko/2/", "/models/juicyneko/", "juicyneko"},
		{"https://rule34video.com/search/mackg4/", "/search/mackg4/", "mackg4"},
	}
	for _, tt := range tests {
		p := NewRule34VideoParser(nil, tt.entry)
		if got := p.listBasePath(); got != tt.wantBasePath {
			t.Errorf("listBasePath(%s) = %s, want %s", tt.entry, got, tt.wantBasePath)
		}
		if got := p.deriveListTitle(); got != tt.wantTitle {
			t.Errorf("deriveListTitle(%s) = %s, want %s", tt.entry, got, tt.wantTitle)
		}
	}
}

func mustDoc(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("failed to parse test html: %v", err)
	}
	return doc
}
