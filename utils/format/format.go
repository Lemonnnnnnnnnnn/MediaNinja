package format

import (
	"fmt"
	"path/filepath"
	"strings"
)

var (
	// Windows系统非法字符
	illegalChars = []rune{'\\', '/', ':', '*', '?', '"', '<', '>', '|'}

	// Windows系统保留名称
	reservedNames = map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
		"COM6": true, "COM7": true, "COM8": true, "COM9": true,
		"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
		"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	}
)

// SanitizeWindowsPath 清理 Windows 路径中的非法字符
func SanitizeWindowsPath(input string) string {
	// 分割路径
	parts := strings.Split(input, string(filepath.Separator))

	// 处理每个部分
	for i, part := range parts {
		parts[i] = sanitizeWindowsFilename(part)
	}

	// 重新组合路径
	return filepath.Join(parts...)
}

// sanitizeWindowsFilename 清理 Windows 文件名中的非法字符
func sanitizeWindowsFilename(input string) string {
	if input == "" {
		return "unnamed"
	}

	// 替换非法字符为下划线
	var result strings.Builder
	for _, r := range input {
		if isIllegalChar(r) {
			result.WriteRune('_')
		} else {
			result.WriteRune(r)
		}
	}

	// 移除开头和结尾的空格和点
	sanitized := strings.TrimFunc(result.String(), func(r rune) bool {
		return r == '.' || r == ' '
	})

	if sanitized == "" {
		return "unnamed"
	}

	// 检查是否是保留名称
	stemName := strings.ToUpper(filepath.Base(sanitized))
	if ext := filepath.Ext(stemName); ext != "" {
		stemName = stemName[:len(stemName)-len(ext)]
	}
	if reservedNames[stemName] {
		sanitized = "_" + sanitized
	}

	// 截断文件名（考虑 Windows 最大路径长度限制）
	const maxLength = 255
	if len(sanitized) > maxLength {
		if ext := filepath.Ext(sanitized); ext != "" {
			// 保留扩展名
			sanitized = sanitized[:maxLength-len(ext)] + ext
		} else {
			sanitized = sanitized[:maxLength]
		}
	}

	return sanitized
}

func isIllegalChar(r rune) bool {
	for _, illegal := range illegalChars {
		if r == illegal {
			return true
		}
	}
	return false
}

// GetFileNameFromURL 从URL中提取文件名
func GetFileNameFromURL(url string) string {
	parts := strings.Split(url, "/")
	return SanitizeWindowsPath(parts[len(parts)-1])
}

// GetFileExt 获取文件扩展名
func GetFileExt(filename string) string {
	return filepath.Ext(filename)
}

// SanitizeFileName 清理文件名中的非法字符
func SanitizeFileName(filename string) string {
	// 实现文件名清理逻辑
	return filename
}

// getFileName 生成文件名
func GetFileName(urlPath string, index int) string {
	// 尝试从URL路径中提取文件名
	parts := strings.Split(urlPath, "/")
	if len(parts) > 0 {
		filename := parts[len(parts)-1]
		if filename != "" {
			return filename
		}
	}

	// 如果无法从URL中提取文件名，则生成一个序号文件名
	return fmt.Sprintf("%03d.mp4", index+1)
}
