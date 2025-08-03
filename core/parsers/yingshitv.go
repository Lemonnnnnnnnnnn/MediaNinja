package parsers

import (
	"encoding/json"
	"fmt"
	"log"
	"MediaNinja/core/request/client"
	"net/url"
	"strings"
)

type YingshitvParser struct {
	client *client.Client
	DefaultDownloader
	url string
}

func NewYingshitvParser(client *client.Client, url string) *YingshitvParser {
	if client == nil {
		log.Printf("Warning: YingshitvParser initialized with nil client")
	}

	return &YingshitvParser{
		client: client,
		url:    url,
	}
}

func (p *YingshitvParser) Parse(_ string) (*ParseResult, error) {
	log.Printf("Starting to parse URL: %s", p.url)
	videoInfo, err := p.fetchVideoInfo()
	if err != nil {
		log.Printf("Error fetching video info: %v", err)
		return nil, fmt.Errorf("failed to fetch video info: %w", err)
	}
	log.Printf("Successfully fetched video info for URL: %s", p.url)

	result := &ParseResult{
		Media: make([]MediaInfo, 0),
		Extra: make(map[string]interface{}),
	}

	result.Title = &videoInfo.Title
	for i, episode := range videoInfo.Episodes {
		url, err := url.Parse(episode.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse URL %s: %w", episode.URL, err)
		}

		result.Media = append(result.Media, MediaInfo{
			URL:       url,
			MediaType: Video,
			Filename:  fmt.Sprintf("第%d集", i+1),
		})

	}

	return result, nil
}

type VideoMeta struct {
	id  string
	tid string
	nid string
}

func (p *YingshitvParser) parseVideoMeta() (VideoMeta, error) {
	log.Printf("Parsing video meta from URL: %s", p.url)
	videoURL, err := url.Parse(p.url)
	if err != nil {
		log.Printf("Error parsing URL %s: %v", p.url, err)
		return VideoMeta{}, fmt.Errorf("failed to parse URL %s: %w", p.url, err)
	}

	// 解析路径参数
	pathSegments := strings.Split(videoURL.Path, "/")
	var id, tid, nid string

	// 遍历路径段查找参数
	for i := 0; i < len(pathSegments); i++ {
		switch pathSegments[i] {
		case "id":
			if i+1 < len(pathSegments) {
				id = pathSegments[i+1]
			}
		case "sid":
			if i+1 < len(pathSegments) {
				tid = pathSegments[i+1]
			}
		case "nid":
			if i+1 < len(pathSegments) {
				nid = pathSegments[i+1]
			}
		}
	}

	return VideoMeta{
		id:  id,
		tid: tid,
		nid: nid,
	}, nil
}

type VideoInfo struct {
	Title    string
	Episodes []VideoEpisode
}

type VideoEpisode struct {
	Title string
	URL   string
}

type VideoInfoResponse struct {
	Code int `json:"code"`
	Data struct {
		VodSources []struct {
			VodPlayList struct {
				UrlCount int `json:"url_count"`
				Urls     []struct {
					Name string `json:"name"`
					Url  string `json:"url"`
				} `json:"urls"`
			} `json:"vod_play_list"`
		} `json:"vod_sources"`
		VodName string `json:"vod_name"`
	} `json:"data"`
}

func (p *YingshitvParser) fetchVideoInfo() (VideoInfo, error) {
	log.Printf("Fetching video info for videoMeta: %v", p.url)
	videoMeta, err := p.parseVideoMeta()
	if err != nil {
		log.Printf("Error parsing video meta: %v", err)
		return VideoInfo{}, fmt.Errorf("failed to parse video meta: %w", err)
	}

	url := fmt.Sprintf("https://api.yingshi.tv/vod/v1/info?id=%s&tid=%s", videoMeta.id, videoMeta.tid)
	log.Printf("Requesting video info from URL: %s", url)

	var videoInfo VideoInfoResponse
	resp, err := p.client.Get(url, nil)

	if err != nil {
		return VideoInfo{}, fmt.Errorf("failed to fetch video info: %w", err)
	}

	if err := json.Unmarshal([]byte(resp), &videoInfo); err != nil {
		return VideoInfo{}, fmt.Errorf("failed to unmarshal video info: %w", err)
	}

	if videoInfo.Code != 0 {
		return VideoInfo{}, fmt.Errorf("failed to fetch video info")
	}

	episodes := []VideoEpisode{}
	for _, source := range videoInfo.Data.VodSources {
		for _, episode := range source.VodPlayList.Urls {
			episodes = append(episodes, VideoEpisode{
				Title: episode.Name,
				URL:   episode.Url,
			})
		}
	}

	return VideoInfo{
		Title:    videoInfo.Data.VodName,
		Episodes: episodes,
	}, nil
}

func (p *YingshitvParser) GetDownloader() Downloader {
	return p
}
