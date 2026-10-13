package config

import (
	"encoding/json"
	"os"
)

// Load 从 JSON 文件读取设置；文件缺失或内容无效时返回 false，
// 由调用方回退到默认值。core 不向 stdout 打印日志，静默回退是既定约定。
func Load(path string) (Config, bool) {
	if path == "" {
		return Config{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, false
	}
	var settings Config
	if err := json.Unmarshal(data, &settings); err != nil {
		return Config{}, false
	}
	if err := settings.Validate(); err != nil {
		return Config{}, false
	}
	return settings, true
}

// Save 把设置原子写入 JSON 文件（临时文件 + rename）；path 为空表示不持久化。
func Save(path string, settings Config) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
