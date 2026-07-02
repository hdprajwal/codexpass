package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config is the optional codexpass configuration file.
type Config struct {
	Version  int            `json:"version"`
	Server   ServerConfig   `json:"server"`
	Models   ModelConfig    `json:"models"`
	Clients  ClientsConfig  `json:"clients"`
	Fallback FallbackConfig `json:"fallback"`
}

type ServerConfig struct {
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Token         string `json:"token"`
	Verbose       bool   `json:"verbose"`
	LogFormat     string `json:"log_format"`
	Metrics       bool   `json:"metrics"`
	StatsPath     string `json:"stats_path"`
	RetryAttempts int    `json:"retry_attempts"`
}

type ModelConfig struct {
	Aliases         map[string]string `json:"aliases"`
	CacheTTLSeconds int64             `json:"cache_ttl_seconds"`
}

type ClientsConfig map[string]ClientConfig

type ClientConfig struct {
	Token              string   `json:"token"`
	AllowedEndpoints   []string `json:"allowed_endpoints"`
	AllowedModels      []string `json:"allowed_models"`
	MaxBodyBytes       int64    `json:"max_body_bytes"`
	RateLimitPerMinute int      `json:"rate_limit_per_minute"`
	AllowFallback      bool     `json:"allow_fallback"`
	Disabled           bool     `json:"disabled"`
}

type FallbackConfig struct {
	Enabled   bool   `json:"enabled"`
	BaseURL   string `json:"base_url"`
	APIKeyEnv string `json:"api_key_env"`
}

// Path returns the default config path, honoring CODEXPASS_CONFIG.
func Path() string {
	if p := os.Getenv("CODEXPASS_CONFIG"); p != "" {
		return p
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "codexpass", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "codexpass", "config.json")
	}
	return filepath.Join(home, ".config", "codexpass", "config.json")
}

// Load loads the default config path.
func Load() (Config, error) {
	return LoadFrom(Path())
}

// LoadFrom loads a JSON config file. Missing files are treated as empty config.
func LoadFrom(path string) (Config, error) {
	if path == "" {
		return Config{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}
	if cfg.Models.Aliases == nil {
		cfg.Models.Aliases = map[string]string{}
	}
	return cfg, nil
}

func (m ModelConfig) CacheTTL() time.Duration {
	if m.CacheTTLSeconds <= 0 {
		return 0
	}
	return time.Duration(m.CacheTTLSeconds) * time.Second
}
