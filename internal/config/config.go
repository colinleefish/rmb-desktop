package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/platform"
	"gopkg.in/yaml.v3"
)

const DefaultAddr = "127.0.0.1:19019"

// Config is the rmb-desktop client and daemon configuration.
type Config struct {
	Addr   string `yaml:"addr"`
	DBPath string `yaml:"db_path"`
}

// Default returns configuration with platform defaults applied.
func Default() (Config, error) {
	dbPath, err := platform.DBPath()
	if err != nil {
		return Config{}, err
	}
	return Config{
		Addr:   DefaultAddr,
		DBPath: dbPath,
	}, nil
}

// Load reads config from path, falling back to defaults for missing fields.
func Load(path string) (Config, error) {
	cfg, err := Default()
	if err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(path) == "" {
		return applyEnv(cfg), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return applyEnv(cfg), nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if strings.TrimSpace(cfg.Addr) == "" {
		cfg.Addr = DefaultAddr
	}
	if strings.TrimSpace(cfg.DBPath) == "" {
		dbPath, err := platform.DBPath()
		if err != nil {
			return Config{}, err
		}
		cfg.DBPath = dbPath
	}
	return applyEnv(cfg), nil
}

// LoadDefault loads from the platform config path.
func LoadDefault() (Config, error) {
	path, err := platform.ConfigPath()
	if err != nil {
		return Config{}, err
	}
	return Load(path)
}

// BaseURL returns the HTTP base URL for API calls (e.g. hook-submit).
func (c Config) BaseURL() string {
	addr := strings.TrimSpace(c.Addr)
	if addr == "" {
		addr = DefaultAddr
	}
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return strings.TrimRight(addr, "/")
	}
	return "http://" + addr
}

func applyEnv(cfg Config) Config {
	if v := strings.TrimSpace(os.Getenv("RMB_ADDR")); v != "" {
		cfg.Addr = v
	}
	if v := strings.TrimSpace(os.Getenv("RMB_DB_PATH")); v != "" {
		cfg.DBPath = v
	}
	return cfg
}
