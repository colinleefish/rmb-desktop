package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/platform"
	"gopkg.in/yaml.v3"
)

const DefaultAddr = "127.0.0.1:19019"

// Config is the rmb-desktop client and daemon configuration.
type Config struct {
	Addr          string         `yaml:"addr"`
	DBPath        string         `yaml:"db_path"`
	LLM           LLMConfig      `yaml:"llm"`
	Embed         EmbedConfig    `yaml:"embed"`
	Pipeline      PipelineConfig `yaml:"pipeline"`
	LaunchAtLogin bool           `yaml:"launch_at_login"`
}

type LLMConfig struct {
	APIBase string        `yaml:"api_base"`
	APIKey  string        `yaml:"api_key"`
	Model   string        `yaml:"model"`
	Timeout time.Duration `yaml:"timeout"`
}

const DefaultLLMTimeout = 300 * time.Second

func (c LLMConfig) HasKey() bool {
	return strings.TrimSpace(c.APIKey) != ""
}

// RequestTimeout returns the per-LLM-call HTTP timeout.
func (c LLMConfig) RequestTimeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return DefaultLLMTimeout
}

type EmbedConfig struct {
	APIBase    string `yaml:"api_base"`
	APIKey     string `yaml:"api_key"`
	Model      string `yaml:"model"`
	Dimensions int    `yaml:"dimensions"`
}

func (c EmbedConfig) HasKey() bool {
	return strings.TrimSpace(c.APIKey) != ""
}

type PipelineConfig struct {
	L1PollInterval    time.Duration `yaml:"l1_poll_interval"`
	L2PollInterval    time.Duration `yaml:"l2_poll_interval"`
	L3PollInterval    time.Duration `yaml:"l3_poll_interval"`
	EmbedPollInterval time.Duration `yaml:"embed_poll_interval"`
	L1EveryN          int           `yaml:"l1_every_n"`
	L1IdleSeconds     time.Duration `yaml:"l1_idle_seconds"`
	L1Warmup          bool          `yaml:"l1_warmup"`
	L2DelayAfterL1    time.Duration `yaml:"l2_delay_after_l1"`
	L1MaxTurns        int           `yaml:"l1_max_turns_per_batch"`
	L1MaxChars        int           `yaml:"l1_max_chars_per_batch"`
	L2MaxAtoms        int           `yaml:"l2_max_atoms_per_batch"`
	L2MaxScenes       int           `yaml:"l2_max_scenes_per_batch"`
	L3MaxAtoms        int           `yaml:"l3_max_atoms_per_batch"`
	EmbedBatchSize    int           `yaml:"embed_batch_size"`
	// Adaptive concurrency (sessions in parallel) with AIMD back pressure.
	L1MinConcurrency int `yaml:"l1_min_concurrency"`
	L1MaxConcurrency int `yaml:"l1_max_concurrency"`
	L2MinConcurrency int `yaml:"l2_min_concurrency"`
	L2MaxConcurrency int `yaml:"l2_max_concurrency"`
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
		LLM: LLMConfig{
			APIBase: "https://api.openai.com/v1",
			Model:   "gpt-4o-mini",
		},
		Embed: EmbedConfig{
			APIBase:    "https://api.openai.com/v1",
			Model:      "text-embedding-3-small",
			Dimensions: 1024,
		},
		Pipeline: PipelineConfig{
			L1PollInterval:    15 * time.Second,
			L2PollInterval:    15 * time.Second,
			L3PollInterval:    5 * time.Minute,
			EmbedPollInterval: 30 * time.Second,
			L1EveryN:          8,
			L1IdleSeconds:     10 * time.Minute,
			L1Warmup:          true,
			L2DelayAfterL1:    90 * time.Second,
			L1MaxTurns:        8,
			L1MaxChars:        24000,
			L2MaxAtoms:        60,
			L2MaxScenes:       8,
			L3MaxAtoms:        60,
			EmbedBatchSize:    32,
			L1MinConcurrency:  1,
			L1MaxConcurrency:  64,
			L2MinConcurrency:  1,
			L2MaxConcurrency:  16,
		},
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
	if cfg.Embed.Dimensions <= 0 {
		cfg.Embed.Dimensions = 1024
	}
	cfg.Pipeline = normalizePipelineConcurrency(cfg.Pipeline)
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

// DistillationEnabled reports whether LLM workers should run (D20: ingest-only without key).
func (c Config) DistillationEnabled() bool {
	return c.LLM.HasKey()
}

func applyEnv(cfg Config) Config {
	if v := strings.TrimSpace(os.Getenv("RMB_ADDR")); v != "" {
		cfg.Addr = v
	}
	if v := strings.TrimSpace(os.Getenv("RMB_DB_PATH")); v != "" {
		cfg.DBPath = v
	}
	if v := strings.TrimSpace(os.Getenv("RMB_LLM_API_KEY")); v != "" {
		cfg.LLM.APIKey = v
	}
	if v := strings.TrimSpace(os.Getenv("RMB_LLM_API_BASE")); v != "" {
		cfg.LLM.APIBase = v
	}
	if v := strings.TrimSpace(os.Getenv("RMB_LLM_MODEL")); v != "" {
		cfg.LLM.Model = v
	}
	if v := strings.TrimSpace(os.Getenv("RMB_EMBED_API_KEY")); v != "" {
		cfg.Embed.APIKey = v
	}
	if v := strings.TrimSpace(os.Getenv("RMB_EMBED_API_BASE")); v != "" {
		cfg.Embed.APIBase = v
	}
	if v := strings.TrimSpace(os.Getenv("RMB_EMBED_MODEL")); v != "" {
		cfg.Embed.Model = v
	}
	cfg.Pipeline = normalizePipelineConcurrency(cfg.Pipeline)
	return cfg
}

func normalizePipelineConcurrency(p PipelineConfig) PipelineConfig {
	if p.L1MinConcurrency <= 0 {
		p.L1MinConcurrency = 1
	}
	if p.L1MaxConcurrency <= 0 {
		p.L1MaxConcurrency = 64
	}
	if p.L1MaxConcurrency < p.L1MinConcurrency {
		p.L1MaxConcurrency = p.L1MinConcurrency
	}
	if p.L2MinConcurrency <= 0 {
		p.L2MinConcurrency = 1
	}
	if p.L2MaxConcurrency <= 0 {
		p.L2MaxConcurrency = 16
	}
	if p.L2MaxConcurrency < p.L2MinConcurrency {
		p.L2MaxConcurrency = p.L2MinConcurrency
	}
	return p
}
