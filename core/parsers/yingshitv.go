package parsers

import (
	"encoding/json"
	"fmt"
	"log"
	"media-crawler/core/request"
	"media-crawler/utils/format"
	"net/url"
)

type YingshitvParser struct {
	client *request.Client
	DefaultDownloader
	url string
}

func NewYingshitvParser(client *request.Client, url string) *YingshitvParser {
	if client == nil {
		log.Printf("Warning: YingshitvParser initialized with nil client")
	}

	return &YingshitvParser{
		client: client,
		url:    url,
	}

}

func (p *YingshitvParser) Parse(_ string) (*ParseResult, error) {
	videoInfo, err := p.fetchVideoInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch video info: %w", err)
	}

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
			Filename:  format.GetFileName(episode.URL, i),
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
	// url like https://www.yingshi.tv/vod/play/id/10855/sid/1/nid/1 ， id = 10855, tid = 1, nid = 1
	videoURL, err := url.Parse(p.url)
	if err != nil {
		return VideoMeta{}, fmt.Errorf("failed to parse URL %s: %w", p.url, err)
	}

	queryParams := videoURL.Query()
	id := queryParams.Get("id")
	tid := queryParams.Get("sid")
	nid := queryParams.Get("nid")

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
			VodPlayList []struct {
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
	videoMeta, err := p.parseVideoMeta()
	if err != nil {
		return VideoInfo{}, fmt.Errorf("failed to parse video meta: %w", err)
	}

	url := fmt.Sprintf("https://api.yingshi.tv/vod/v1/info?id=%s&tid=%s", videoMeta.id, videoMeta.tid)

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
		for _, episode := range source.VodPlayList {
			for _, url := range episode.Urls {
				episodes = append(episodes, VideoEpisode{
					Title: url.Name,
					URL:   url.Url,
				})
			}
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
