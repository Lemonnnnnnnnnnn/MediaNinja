//go:build !windows

package downloader

import "os/exec"

func configureFFmpegCommand(cmd *exec.Cmd) {}
