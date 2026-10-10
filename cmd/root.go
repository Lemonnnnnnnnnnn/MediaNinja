package cmd

import (
	"MediaNinja/core/application"
	"MediaNinja/core/events"
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"MediaNinja/core/config"

	"github.com/spf13/cobra"
)

var (
	cfg = config.New()
)

var rootCmd = &cobra.Command{
	Use:   "MediaNinja",
	Short: "A media crawler for websites",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := application.Execute(cmd.Context(), application.Request{URL: cfg.URL, Options: *cfg}, cliObserver)
		return err
	},
}

func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
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

func cliObserver(e events.Event) {
	switch e.Type {
	case "progress":
		if time.Since(lastProgress[e.File]) >= time.Second || (e.Total > 0 && e.Completed == e.Total) {
			fmt.Fprintf(os.Stderr, "%s: %d / %d %s\n", e.File, e.Completed, e.Total, e.Unit)
			lastProgress[e.File] = time.Now()
		}
	case "log":
		fmt.Println(e.Message)
	case "analyzing", "downloading":
		fmt.Println(e.Type)
	case "file_completed":
		fmt.Println("Downloaded:", e.File)
	case "file_failed":
		fmt.Fprintln(os.Stderr, "Failed:", e.File, e.Error)
	}
}

var lastProgress = map[string]time.Time{}
