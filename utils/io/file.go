package io

import (
	"fmt"
	"media-crawler/utils/format"
	"os"
	"path/filepath"
)

// Manager handles IO operations
type Manager struct {
	baseDir string
}

// NewManager creates a new IO Manager instance
func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir: baseDir,
	}
}

// WriteFile writes data to a file in the specified subdirectory
func (m *Manager) WriteFile(data interface{}, filename, subDir string, titleDir string) error {
	if filename == "" {
		return fmt.Errorf("invalid filename")
	}

	savePath := filepath.Join(m.baseDir, titleDir, subDir, format.SanitizeWindowsPath(filename))

	if err := m.EnsureDir(savePath); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	var fileData []byte
	switch v := data.(type) {
	case []byte:
		fileData = v
	case string:
		fileData = []byte(v)
	default:
		return fmt.Errorf("unsupported content type")
	}

	return os.WriteFile(savePath, fileData, 0644)
}

// ensureDir creates all necessary parent directories for a file
func (m *Manager) EnsureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0755)
}
