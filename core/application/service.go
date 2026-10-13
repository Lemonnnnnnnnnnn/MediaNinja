// Package application exposes reusable use cases independent of CLI and HTTP.
package application

import (
	"MediaNinja/core/config"
	"MediaNinja/core/crawler"
	"MediaNinja/core/events"
	"MediaNinja/core/parsers"
	"MediaNinja/core/request/client"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// BatchRequest 表示一次批次下载：在选定批次源上按名称列表枚举条目。
type BatchRequest struct {
	Site  string   `json:"site"`
	Names []string `json:"names"`
}

type Request struct {
	URL   string        `json:"url,omitempty"`
	Batch *BatchRequest `json:"batch,omitempty"`
	// SkipFiles 列出上次已成功收集的文件名；重跑时跳过这些文件的下载。
	SkipFiles []string      `json:"skip_files,omitempty"`
	Options   config.Config `json:"options"`
}
type Result struct {
	Files []crawler.FileResult `json:"files"`
}

func (r Request) Validate() error {
	if err := r.Options.Validate(); err != nil {
		return err
	}
	if (r.URL == "") == (r.Batch == nil) {
		return fmt.Errorf("provide either url or batch")
	}
	if r.URL != "" {
		u, err := url.Parse(r.URL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("invalid HTTP URL")
		}
	}
	if r.Batch != nil {
		if _, err := parsers.GetBatchSource(r.Batch.Site); err != nil {
			return err
		}
		if len(r.Batch.Names) == 0 {
			return fmt.Errorf("batch requires at least one name")
		}
		for _, name := range r.Batch.Names {
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("name cannot be empty")
			}
		}
	}
	for _, file := range r.SkipFiles {
		if strings.TrimSpace(file) == "" {
			return fmt.Errorf("skip file cannot be empty")
		}
	}
	if r.Options.ProxyURL != "" {
		u, err := url.Parse(r.Options.ProxyURL)
		if err != nil || u.Host == "" || u.Scheme != "http" {
			return fmt.Errorf("proxy must be an HTTP URL")
		}
	}
	return nil
}

// skipSet 返回需要跳过下载的文件名集合。
func (r Request) skipSet() map[string]bool {
	if len(r.SkipFiles) == 0 {
		return nil
	}
	skip := make(map[string]bool, len(r.SkipFiles))
	for _, file := range r.SkipFiles {
		skip[file] = true
	}
	return skip
}

func Execute(ctx context.Context, r Request, observer events.Observer) (Result, error) {
	result := Result{Files: []crawler.FileResult{}}
	if err := r.Validate(); err != nil {
		return result, err
	}

	skip := r.skipSet()
	var failures []error
	run := func(target string) {
		c := crawler.NewCrawler(&r.Options)
		c.SkipFiles = skip
		execution, err := c.Run(ctx, target, observer)
		result.Files = append(result.Files, execution.Files...)
		if err != nil {
			failures = append(failures, err)
		}
	}
	if r.Batch == nil {
		run(r.URL)
	} else {
		source, err := parsers.GetBatchSource(r.Batch.Site)
		if err != nil {
			return result, err
		}
		c := client.NewClient(r.Options.ProxyURL, r.Options.MaxRetries, r.Options.RetryDelay)
		c.Context = ctx
		for _, name := range r.Batch.Names {
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			target, err := source.Resolve(c, name)
			if err != nil {
				failures = append(failures, err)
				continue
			}
			run(target)
		}
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return result, errors.Join(failures...)
}
