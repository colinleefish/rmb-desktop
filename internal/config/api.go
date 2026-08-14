package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/colinleefish/rmb-desktop/internal/platform"
)

// View is the config shape returned to the web UI (secrets masked).
type View struct {
	Addr                 string         `json:"addr"`
	DBPath               string         `json:"db_path"`
	ConfigPath           string         `json:"config_path"`
	LLM                  LLMView        `json:"llm"`
	Embed                EmbedView      `json:"embed"`
	Pipeline             PipelineView   `json:"pipeline"`
	LaunchAtLogin        bool           `json:"launch_at_login"`
	DistillationEnabled  bool           `json:"distillation_enabled"`
}

type LLMView struct {
	APIBase      string `json:"api_base"`
	APIKeySet    bool   `json:"api_key_set"`
	APIKeySuffix string `json:"api_key_suffix,omitempty"`
	Model        string `json:"model"`
	Timeout      string `json:"timeout"`
}

type EmbedView struct {
	APIBase      string `json:"api_base"`
	APIKeySet    bool   `json:"api_key_set"`
	APIKeySuffix string `json:"api_key_suffix,omitempty"`
	Model        string `json:"model"`
	Dimensions   int    `json:"dimensions"`
}

type PipelineView struct {
	L1PollInterval    string `json:"l1_poll_interval"`
	L2PollInterval    string `json:"l2_poll_interval"`
	L3PollInterval    string `json:"l3_poll_interval"`
	EmbedPollInterval string `json:"embed_poll_interval"`
	L1EveryN          int    `json:"l1_every_n"`
	L1IdleSeconds     string `json:"l1_idle_seconds"`
	L1Warmup          bool   `json:"l1_warmup"`
	L2DelayAfterL1    string `json:"l2_delay_after_l1"`
	L1MaxTurns        int    `json:"l1_max_turns_per_batch"`
	L1MaxChars        int    `json:"l1_max_chars_per_batch"`
	L2MaxAtoms        int    `json:"l2_max_atoms_per_batch"`
	L2MaxScenes       int    `json:"l2_max_scenes_per_batch"`
	L3MaxAtoms        int    `json:"l3_max_atoms_per_batch"`
	EmbedBatchSize    int    `json:"embed_batch_size"`
	L1MinConcurrency  int    `json:"l1_min_concurrency"`
	L1MaxConcurrency  int    `json:"l1_max_concurrency"`
	L2MinConcurrency  int    `json:"l2_min_concurrency"`
	L2MaxConcurrency  int    `json:"l2_max_concurrency"`
}

// UpdateRequest is the PUT body from the settings page.
type UpdateRequest struct {
	Addr          *string           `json:"addr,omitempty"`
	LLM           *LLMUpdate        `json:"llm,omitempty"`
	Embed         *EmbedUpdate      `json:"embed,omitempty"`
	Pipeline      *PipelineUpdate   `json:"pipeline,omitempty"`
	LaunchAtLogin *bool             `json:"launch_at_login,omitempty"`
}

type LLMUpdate struct {
	APIBase string  `json:"api_base"`
	APIKey  string  `json:"api_key"`
	Model   string  `json:"model"`
	Timeout *string `json:"timeout"`
}

type EmbedUpdate struct {
	APIBase    string `json:"api_base"`
	APIKey     string `json:"api_key"`
	Model      string `json:"model"`
	Dimensions *int   `json:"dimensions"`
}

type PipelineUpdate struct {
	L1PollInterval    *string `json:"l1_poll_interval"`
	L2PollInterval    *string `json:"l2_poll_interval"`
	L3PollInterval    *string `json:"l3_poll_interval"`
	EmbedPollInterval *string `json:"embed_poll_interval"`
	L1EveryN          *int    `json:"l1_every_n"`
	L1IdleSeconds     *string `json:"l1_idle_seconds"`
	L1Warmup          *bool   `json:"l1_warmup"`
	L2DelayAfterL1    *string `json:"l2_delay_after_l1"`
	L1MaxTurns        *int    `json:"l1_max_turns_per_batch"`
	L1MaxChars        *int    `json:"l1_max_chars_per_batch"`
	L2MaxAtoms        *int    `json:"l2_max_atoms_per_batch"`
	L2MaxScenes       *int    `json:"l2_max_scenes_per_batch"`
	L3MaxAtoms        *int    `json:"l3_max_atoms_per_batch"`
	EmbedBatchSize    *int    `json:"embed_batch_size"`
	L1MinConcurrency  *int    `json:"l1_min_concurrency"`
	L1MaxConcurrency  *int    `json:"l1_max_concurrency"`
	L2MinConcurrency  *int    `json:"l2_min_concurrency"`
	L2MaxConcurrency  *int    `json:"l2_max_concurrency"`
}

func ToView(cfg Config, configPath string) View {
	return View{
		Addr:                cfg.Addr,
		DBPath:              cfg.DBPath,
		ConfigPath:          configPath,
		DistillationEnabled: cfg.DistillationEnabled(),
		LLM: LLMView{
			APIBase:      cfg.LLM.APIBase,
			APIKeySet:    cfg.LLM.HasKey(),
			APIKeySuffix: keySuffix(cfg.LLM.APIKey),
			Model:        cfg.LLM.Model,
			Timeout:      cfg.LLM.RequestTimeout().String(),
		},
		Embed: EmbedView{
			APIBase:      cfg.Embed.APIBase,
			APIKeySet:    cfg.Embed.HasKey(),
			APIKeySuffix: keySuffix(cfg.Embed.APIKey),
			Model:        cfg.Embed.Model,
			Dimensions:   cfg.Embed.Dimensions,
		},
		Pipeline: pipelineToView(cfg.Pipeline),
		LaunchAtLogin: cfg.LaunchAtLogin,
	}
}

func pipelineToView(p PipelineConfig) PipelineView {
	p = normalizePipelineConcurrency(p)
	return PipelineView{
		L1PollInterval:    p.L1PollInterval.String(),
		L2PollInterval:    p.L2PollInterval.String(),
		L3PollInterval:    p.L3PollInterval.String(),
		EmbedPollInterval: p.EmbedPollInterval.String(),
		L1EveryN:          p.L1EveryN,
		L1IdleSeconds:     p.L1IdleSeconds.String(),
		L1Warmup:          p.L1Warmup,
		L2DelayAfterL1:    p.L2DelayAfterL1.String(),
		L1MaxTurns:        p.L1MaxTurns,
		L1MaxChars:        p.L1MaxChars,
		L2MaxAtoms:        p.L2MaxAtoms,
		L2MaxScenes:       p.L2MaxScenes,
		L3MaxAtoms:        p.L3MaxAtoms,
		EmbedBatchSize:    p.EmbedBatchSize,
		L1MinConcurrency:  p.L1MinConcurrency,
		L1MaxConcurrency:  p.L1MaxConcurrency,
		L2MinConcurrency:  p.L2MinConcurrency,
		L2MaxConcurrency:  p.L2MaxConcurrency,
	}
}

// ApplyUpdate merges a PUT request into cfg. Empty api_key fields keep existing keys.
func ApplyUpdate(cfg Config, req UpdateRequest) (Config, error) {
	if req.Addr != nil {
		cfg.Addr = strings.TrimSpace(*req.Addr)
	}
	if req.LLM != nil {
		if v := strings.TrimSpace(req.LLM.APIBase); v != "" {
			cfg.LLM.APIBase = v
		}
		if v := strings.TrimSpace(req.LLM.Model); v != "" {
			cfg.LLM.Model = v
		}
		if k := strings.TrimSpace(req.LLM.APIKey); k != "" {
			cfg.LLM.APIKey = k
		}
		if req.LLM.Timeout != nil {
			v := strings.TrimSpace(*req.LLM.Timeout)
			if v == "" {
				cfg.LLM.Timeout = 0
			} else {
				d, err := time.ParseDuration(v)
				if err != nil {
					return Config{}, fmt.Errorf("invalid llm.timeout: %w", err)
				}
				if d < time.Second {
					return Config{}, fmt.Errorf("llm.timeout must be at least 1s")
				}
				cfg.LLM.Timeout = d
			}
		}
	}
	if req.Embed != nil {
		if v := strings.TrimSpace(req.Embed.APIBase); v != "" {
			cfg.Embed.APIBase = v
		}
		if v := strings.TrimSpace(req.Embed.Model); v != "" {
			cfg.Embed.Model = v
		}
		if k := strings.TrimSpace(req.Embed.APIKey); k != "" {
			cfg.Embed.APIKey = k
		}
		if req.Embed.Dimensions != nil && *req.Embed.Dimensions > 0 {
			cfg.Embed.Dimensions = *req.Embed.Dimensions
		}
	}
	if req.Pipeline != nil {
		var err error
		cfg.Pipeline, err = applyPipelineUpdate(cfg.Pipeline, *req.Pipeline)
		if err != nil {
			return Config{}, err
		}
	}
	if req.LaunchAtLogin != nil {
		cfg.LaunchAtLogin = *req.LaunchAtLogin
	}
	return cfg, nil
}

func applyPipelineUpdate(p PipelineConfig, u PipelineUpdate) (PipelineConfig, error) {
	var err error
	if u.L1PollInterval != nil {
		p.L1PollInterval, err = time.ParseDuration(strings.TrimSpace(*u.L1PollInterval))
		if err != nil {
			return p, fmt.Errorf("l1_poll_interval: %w", err)
		}
	}
	if u.L2PollInterval != nil {
		p.L2PollInterval, err = time.ParseDuration(strings.TrimSpace(*u.L2PollInterval))
		if err != nil {
			return p, fmt.Errorf("l2_poll_interval: %w", err)
		}
	}
	if u.L3PollInterval != nil {
		p.L3PollInterval, err = time.ParseDuration(strings.TrimSpace(*u.L3PollInterval))
		if err != nil {
			return p, fmt.Errorf("l3_poll_interval: %w", err)
		}
	}
	if u.EmbedPollInterval != nil {
		p.EmbedPollInterval, err = time.ParseDuration(strings.TrimSpace(*u.EmbedPollInterval))
		if err != nil {
			return p, fmt.Errorf("embed_poll_interval: %w", err)
		}
	}
	if u.L1IdleSeconds != nil {
		p.L1IdleSeconds, err = time.ParseDuration(strings.TrimSpace(*u.L1IdleSeconds))
		if err != nil {
			return p, fmt.Errorf("l1_idle_seconds: %w", err)
		}
	}
	if u.L2DelayAfterL1 != nil {
		p.L2DelayAfterL1, err = time.ParseDuration(strings.TrimSpace(*u.L2DelayAfterL1))
		if err != nil {
			return p, fmt.Errorf("l2_delay_after_l1: %w", err)
		}
	}
	if u.L1EveryN != nil {
		p.L1EveryN = *u.L1EveryN
	}
	if u.L1Warmup != nil {
		p.L1Warmup = *u.L1Warmup
	}
	if u.L1MaxTurns != nil {
		p.L1MaxTurns = *u.L1MaxTurns
	}
	if u.L1MaxChars != nil {
		p.L1MaxChars = *u.L1MaxChars
	}
	if u.L2MaxAtoms != nil {
		p.L2MaxAtoms = *u.L2MaxAtoms
	}
	if u.L2MaxScenes != nil {
		p.L2MaxScenes = *u.L2MaxScenes
	}
	if u.L3MaxAtoms != nil {
		p.L3MaxAtoms = *u.L3MaxAtoms
	}
	if u.EmbedBatchSize != nil {
		p.EmbedBatchSize = *u.EmbedBatchSize
	}
	if u.L1MinConcurrency != nil {
		p.L1MinConcurrency = *u.L1MinConcurrency
	}
	if u.L1MaxConcurrency != nil {
		p.L1MaxConcurrency = *u.L1MaxConcurrency
	}
	if u.L2MinConcurrency != nil {
		p.L2MinConcurrency = *u.L2MinConcurrency
	}
	if u.L2MaxConcurrency != nil {
		p.L2MaxConcurrency = *u.L2MaxConcurrency
	}
	return normalizePipelineConcurrency(p), nil
}

// Save writes cfg to path, creating parent directories as needed.
func Save(path string, cfg Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// ResolvePath returns an explicit path or the platform default.
func ResolvePath(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, nil
	}
	return platform.ConfigPath()
}

func keySuffix(key string) string {
	key = strings.TrimSpace(key)
	if len(key) < 2 {
		return ""
	}
	return key[len(key)-2:]
}
