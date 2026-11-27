package store

import (
	"SysInitPro/internal/models"
	"encoding/json"
	"os"
	"path/filepath"
)

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "SysInitPro")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(appDir, "config.json"), nil
}

func Load() models.AppConfig {
	p, err := configPath()
	if err != nil {
		return models.AppConfig{}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return models.AppConfig{}
	}
	var cfg models.AppConfig
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

func Save(cfg models.AppConfig) error {
	p, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}
