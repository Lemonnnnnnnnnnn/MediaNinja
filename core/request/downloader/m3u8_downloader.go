package downloader

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/grafov/m3u8"
)

type M3U8Downloader struct {
	client       ClientInterface
	output       string
	opts         *RequestOption
	showProgress bool
}

// 用于保存下载进度的结构
type downloadState struct {
	DownloadedSegments map[string]bool `json:"downloaded_segments"`
	TotalSegments      int             `json:"total_segments"`
}

func NewM3U8Downloader(client ClientInterface, output string, opts *RequestOption, showProgress bool) *M3U8Downloader {
	return &M3U8Downloader{
		client:       client,
		output:       output,
		opts:         opts,
		showProgress: showProgress,
	}
}

func (m *M3U8Downloader) DownloadFromURL(m3u8URL string) error {
	// 创建临时文件用于存储 ts 文件
	tempFile := m.output + ".ts"
	stateFile := m.getStateFilePath()
	defer os.Remove(stateFile) // 下载完成后删除状态文件

	// 检查是否存在未完成的下载
	var flags int
	if _, err := os.Stat(tempFile); err == nil {
		flags = os.O_WRONLY | os.O_APPEND
	} else {
		flags = os.O_CREATE | os.O_WRONLY
	}

	// 打开或创建输出文件
	outFile, err := os.OpenFile(tempFile, flags, 0644)
	if err != nil {
		return fmt.Errorf("failed to create/open temp file: %w", err)
	}
	defer outFile.Close()

	// 下载内容
	if err := m.downloadM3U8Content(m3u8URL, outFile); err != nil {
		return err
	}

	// 转换为 MP4
	if err := m.convertToMP4(tempFile, m.output); err != nil {
		return err
	}

	// 清理临时文件
	os.Remove(tempFile)
	return nil
}

func (m *M3U8Downloader) downloadM3U8Content(m3u8URL string, outFile *os.File) error {
	log.Printf("Starting download from URL: %s", m3u8URL)

	// 获取 m3u8 内容
	resp, err := m.client.GetStream("GET", m3u8URL, m.opts, nil)
	if err != nil {
		return fmt.Errorf("failed to get m3u8: %w", err)
	}
	defer resp.Body.Close()

	// 解析 m3u8 文件
	playlist, listType, err := m3u8.DecodeFrom(resp.Body, true)
	if err != nil {
		return fmt.Errorf("failed to decode m3u8: %w", err)
	}

	// 处理不同类型的播放列表
	switch listType {
	case m3u8.MEDIA:
		mediapl := playlist.(*m3u8.MediaPlaylist)
		return m.downloadSegments(mediapl, outFile)
	case m3u8.MASTER:
		return fmt.Errorf("master playlist not supported yet")
	default:
		return fmt.Errorf("unknown playlist type")
	}
}

func (m *M3U8Downloader) downloadSegments(playlist *m3u8.MediaPlaylist, outFile *os.File) error {
	// 计算总片段数
	var totalSegments int
	for _, segment := range playlist.Segments {
		if segment != nil {
			totalSegments++
		}
	}

	// 获取或创建下载状态
	state := m.loadDownloadState()
	if state == nil {
		state = &downloadState{
			DownloadedSegments: make(map[string]bool),
			TotalSegments:      totalSegments,
		}
	}

	// 创建进度显示器
	var progress *DownloadProgress
	if m.showProgress {
		progress = NewDownloadProgress(path.Base(m.output), int64(totalSegments), int64(len(state.DownloadedSegments)))
	}

	// 下载所有片段
	for _, segment := range playlist.Segments {
		if segment == nil {
			continue
		}

		// 如果该片段已下载，跳过
		if state.DownloadedSegments[segment.URI] {
			continue
		}

		// 下载分片
		resp, err := m.client.GetStream("GET", segment.URI, m.opts, nil)
		if err != nil {
			if progress != nil {
				progress.Fail(err)
			}
			return fmt.Errorf("failed to download segment %s: %w", segment.URI, err)
		}

		// 获取文件当前位置用于写入
		currentPos, err := outFile.Seek(0, io.SeekEnd)
		if err != nil {
			resp.Body.Close()
			return fmt.Errorf("failed to seek file: %w", err)
		}

		// 写入分片数据
		_, err = io.Copy(outFile, resp.Body)
		resp.Body.Close()
		if err != nil {
			if progress != nil {
				progress.Fail(err)
			}
			// 如果写入失败，回滚文件位置
			outFile.Truncate(currentPos)
			return fmt.Errorf("failed to write segment: %w", err)
		}

		// 标记该片段已下载
		state.DownloadedSegments[segment.URI] = true
		m.saveDownloadState(state)

		if progress != nil {
			progress.Update(1)
		}
	}

	if progress != nil {
		progress.Success()
	}
	return nil
}

func (m *M3U8Downloader) convertToMP4(inputFile, outputFile string) error {
	// 检查 ffmpeg 是否可用
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found: %w", err)
	}

	// 如果输出文件已经是 mp4 格式，则不需要转换
	if !strings.HasSuffix(strings.ToLower(outputFile), ".mp4") {
		outputFile = outputFile + ".mp4"
	}

	// 构建 ffmpeg 命令
	cmd := exec.Command("ffmpeg",
		"-i", inputFile,
		"-c", "copy", // 直接复制流，不重新编码
		"-bsf:a", "aac_adtstoasc", // 修复音频
		"-y", // 覆盖已存在的文件
		outputFile,
	)

	// 执行命令
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg conversion failed: %s, %w", string(output), err)
	}

	log.Printf("Successfully converted to MP4: %s", outputFile)
	return nil
}

func (m *M3U8Downloader) getStateFilePath() string {
	return m.output + ".state.json"
}

func (m *M3U8Downloader) loadDownloadState() *downloadState {
	stateFile := m.getStateFilePath()
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return nil
	}

	var state downloadState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	return &state
}

func (m *M3U8Downloader) saveDownloadState(state *downloadState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(m.getStateFilePath(), data, 0644)
}
