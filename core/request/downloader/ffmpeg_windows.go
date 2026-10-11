package downloader

import (
	"os/exec"
	"syscall"
)

func configureFFmpegCommand(cmd *exec.Cmd) {
	// ffmpeg is a console application; desktop downloads must not flash a terminal.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
}
