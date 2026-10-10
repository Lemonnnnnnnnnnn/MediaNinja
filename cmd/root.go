package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"MediaNinja/core/config"
	"MediaNinja/core/crawler"
	"MediaNinja/core/parsers"

	"github.com/spf13/cobra"
)

var (
	cfg = config.New()
)

var rootCmd = &cobra.Command{
	Use:   "MediaNinja",
	Short: "A media crawler for websites",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// 确保输出目录存在
		if cfg.OutputDir == "" {
			cfg.OutputDir = "downloads"
		}

		// 转换为绝对路径
		absPath, err := filepath.Abs(cfg.OutputDir)
		if err != nil {
			fmt.Printf("Error resolving output path: %v\n", err)
			os.Exit(1)
		}
		cfg.OutputDir = absPath

		// 创建输出目录
		if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
			fmt.Printf("Error creating output directory: %v\n", err)
			os.Exit(1)
		}

		// rule34 列表页抓取选项
		if cfg.MaxPages < 1 {
			cfg.MaxPages = 1
		}
		parsers.Rule34Options.MaxPages = cfg.MaxPages
		parsers.Rule34Options.MaxVideos = cfg.MaxVideos
		parsers.Rule34Options.Concurrency = cfg.Concurrency
	},
	Run: func(cmd *cobra.Command, args []string) {
		c := crawler.NewCrawler(cfg)
		if err := c.Start(cfg.URL); err != nil {
			fmt.Printf("Crawler error: %v\n", err)
			os.Exit(1)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// 必需参数
	rootCmd.Flags().StringVarP(&cfg.URL, "url", "u", "", "Target URL to crawl")
	rootCmd.MarkFlagRequired("url")

	// 可选参数（Persistent，子命令共用）
	rootCmd.PersistentFlags().StringVarP(&cfg.ProxyURL, "proxy", "p", "", "Proxy URL (optional)")
	rootCmd.PersistentFlags().IntVarP(&cfg.Concurrency, "concurrency", "c", 5, "Number of concurrent downloads")
	rootCmd.PersistentFlags().StringVarP(&cfg.OutputDir, "output", "o", "downloads", "Output directory for downloaded files")

	// Add new flags for retry configuration
	rootCmd.PersistentFlags().IntVarP(&cfg.MaxRetries, "max-retries", "r", 3, "Maximum number of retry attempts for downloads")
	rootCmd.PersistentFlags().IntVarP(&cfg.RetryDelay, "retry-delay", "d", 5, "Delay between retry attempts in seconds")
}
