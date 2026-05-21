package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type RuntimeConfig struct {
	Backend BackendRuntimeConfig `json:"backend"`
	Frontend FrontendRuntimeConfig `json:"frontend"`
}

type BackendRuntimeConfig struct {
	Port       int    `json:"port"`
	DataPath   string `json:"dataPath"`
	PresetsPath string `json:"presetsPath"`
}

type FrontendRuntimeConfig struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	APITarget string `json:"apiTarget"`
}

func LoadRuntimeConfig(path string) (RuntimeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("read runtime config: %w", err)
	}

	var cfg RuntimeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return RuntimeConfig{}, fmt.Errorf("parse runtime config: %w", err)
	}
	if cfg.Backend.Port == 0 {
		cfg.Backend.Port = 18130
	}
	if cfg.Frontend.Port == 0 {
		cfg.Frontend.Port = 5173
	}
	if cfg.Frontend.Host == "" {
		cfg.Frontend.Host = "127.0.0.1"
	}
	return cfg, nil
}

func ResolveRuntimeConfigPath(candidates []string) string {
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return filepath.Clean("config/app.json")
}

func ResolveRelativeToConfig(configPath string, target string) string {
	if target == "" {
		return ""
	}
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	baseDir := filepath.Dir(configPath)
	return filepath.Clean(filepath.Join(baseDir, target))
}
