package cmd

import (
	"strings"

	"MediaNinja/core/application"

	"github.com/spf13/cobra"
)

var rule34Names string

var rule34Cmd = &cobra.Command{
	Use:   "rule34",
	Short: "Batch download videos from rule34video.com by creator or search keyword",
	RunE: func(cmd *cobra.Command, args []string) error {
		names := []string{}
		for _, name := range strings.Split(rule34Names, ",") {
			if name = strings.TrimSpace(name); name != "" {
				names = append(names, name)
			}
		}
		_, err := application.Execute(cmd.Context(), application.Request{
			Batch:   &application.BatchRequest{Site: "rule34video", Names: names},
			Options: *cfg,
		}, cliObserver)
		return err
	},
}

func init() {
	rootCmd.AddCommand(rule34Cmd)

	rule34Cmd.Flags().StringVarP(&rule34Names, "names", "n", "", "Comma-separated creator names or search keywords")
	rule34Cmd.MarkFlagRequired("names")
	rule34Cmd.Flags().IntVar(&cfg.MaxPages, "pages", 1, "Max list pages to fetch per name (models pages support pagination)")
	rule34Cmd.Flags().IntVar(&cfg.MaxVideos, "limit", 0, "Max videos to download per name, 0 for no limit")
}
