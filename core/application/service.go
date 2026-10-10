// Package application exposes reusable use cases independent of CLI and HTTP.
package application

import (
	"MediaNinja/core/config"
	"MediaNinja/core/crawler"
	"MediaNinja/core/events"
	"MediaNinja/core/request/client"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type Request struct {
	URL     string        `json:"url,omitempty"`
	Names   []string      `json:"names,omitempty"`
	Options config.Config `json:"options"`
}
type Result struct {
	Files []crawler.FileResult `json:"files"`
}

func (r Request) Validate() error {
	if err := r.Options.Validate(); err != nil {
		return err
	}
	if (r.URL == "") == (len(r.Names) == 0) {
		return fmt.Errorf("provide either url or names")
	}
	if r.URL != "" {
		u, err := url.Parse(r.URL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("invalid HTTP URL")
		}
	}
	for _, name := range r.Names {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("name cannot be empty")
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
func Execute(ctx context.Context, r Request, observer events.Observer) (Result, error) {
	result := Result{Files: []crawler.FileResult{}}
	if err := r.Validate(); err != nil {
		return result, err
	}

	var failures []error
	run := func(target string) {
		execution, err := crawler.NewCrawler(&r.Options).Run(ctx, target, observer)
		result.Files = append(result.Files, execution.Files...)
		if err != nil {
			failures = append(failures, err)
		}
	}
	if len(r.Names) == 0 {
		run(r.URL)
	} else {
		c := client.NewClient(r.Options.ProxyURL, r.Options.MaxRetries, r.Options.RetryDelay)
		c.Context = ctx
		for _, name := range r.Names {
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			escaped := url.PathEscape(strings.TrimSpace(name))
			target := "https://rule34video.com/models/" + escaped + "/"
			response, err := c.GetStream("GET", target, nil, nil)
			if err != nil {
				failures = append(failures, fmt.Errorf("%s: %w", name, err))
				continue
			}
			status := response.StatusCode
			response.Body.Close()
			if status == 404 {
				target = "https://rule34video.com/search/" + escaped + "/"
			} else if status != 200 {
				failures = append(failures, fmt.Errorf("%s: author page HTTP status %d", name, status))
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
