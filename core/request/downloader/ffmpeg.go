package downloader

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func findFFmpeg() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	return resolveFFmpeg(os.Getenv("MEDIANINJA_FFMPEG"), executable, exec.LookPath)
}

func resolveFFmpeg(explicit, executable string, lookup func(string) (string, error)) (string, error) {
	check := func(path string) (string, error) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(absolute)
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("ffmpeg path is not a file: %s", absolute)
		}
		return absolute, nil
	}
	if explicit != "" {
		path, err := check(explicit)
		if err != nil {
			return "", fmt.Errorf("invalid MEDIANINJA_FFMPEG: %w", err)
		}
		return path, nil
	}
	name := "ffmpeg"
	if filepath.Ext(executable) == ".exe" {
		name += ".exe"
	}
	path, err := check(filepath.Join(filepath.Dir(executable), name))
	if err == nil {
		return path, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("unable to use bundled ffmpeg: %w", err)
	}
	path, err = lookup("ffmpeg")
	if err != nil {
		return "", fmt.Errorf("ffmpeg not found beside executable or in PATH; set MEDIANINJA_FFMPEG to override: %w", err)
	}
	return path, nil
}
