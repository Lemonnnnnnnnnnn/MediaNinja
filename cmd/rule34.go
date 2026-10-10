package cmd

import (
	"fmt"
	"strings"

	"MediaNinja/core/crawler"
	"MediaNinja/core/request/client"
	"MediaNinja/utils/logger"

	"github.com/spf13/cobra"
)

var rule34Names string

// rule34TargetURL 优先使用作者页（支持路径翻页），不存在（404）则回退到搜索页
func rule34TargetURL(c *client.Client, name string) string {
	modelsURL := "https://rule34video.com/models/" + name + "/"

	resp, err := c.GetStream("GET", modelsURL, nil, nil)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			return modelsURL
		}
	}
	return "https://rule34video.com/search/" + name + "/"
}

var rule34Cmd = &cobra.Command{
	Use:   "rule34",
	Short: "Batch download videos from rule34video.com by creator or search keyword",
	Run: func(cmd *cobra.Command, args []string) {
		c := client.NewClient(cfg.ProxyURL, cfg.MaxRetries, cfg.RetryDelay)

		for _, name := range strings.Split(rule34Names, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}

			target := rule34TargetURL(c, name)
			logger.Info(fmt.Sprintf("rule34: crawling %s for \"%s\"", target, name))

			// 每个名字独立的 crawler，输出到 downloads/{name}/ 下
			nameCfg := *cfg
			nameCfg.URL = target
			cw := crawler.NewCrawler(&nameCfg)
			if err := cw.Start(target); err != nil {
				logger.Error(fmt.Sprintf("rule34: failed to crawl \"%s\": %v", name, err))
				continue
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(rule34Cmd)

	rule34Cmd.Flags().StringVarP(&rule34Names, "names", "n", "", "Comma-separated creator names or search keywords")
	rule34Cmd.MarkFlagRequired("names")
	rule34Cmd.Flags().IntVar(&cfg.MaxPages, "pages", 1, "Max list pages to fetch per name (models pages support pagination)")
	rule34Cmd.Flags().IntVar(&cfg.MaxVideos, "limit", 0, "Max videos to download per name, 0 for no limit")
}
