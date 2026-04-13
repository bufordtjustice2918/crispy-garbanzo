// Package config provides typed, validated configuration for all Clawgress services.
//
// Configuration is loaded from a JSON file. Unknown fields are rejected.
// All fields have sane defaults — a missing config file starts with defaults.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Config is the top-level service configuration.
type Config struct {
	Gateway  GatewayConfig  `json:"gateway"`
	AdminAPI AdminAPIConfig `json:"admin_api"`
	Files    FilesConfig    `json:"files"`
}

// GatewayConfig controls the proxy listener.
type GatewayConfig struct {
	Listen       string `json:"listen"`         // default ":3128"
	ReadTimeout  int    `json:"read_timeout_s"` // seconds, default 60
	WriteTimeout int    `json:"write_timeout_s"`
}

// AdminAPIConfig controls the admin API listener.
type AdminAPIConfig struct {
	Listen   string `json:"listen"`    // default ":8080"
	UIPath   string `json:"ui_path"`   // default "/ui/"
	StateDir string `json:"state_dir"` // default "/var/lib/clawgress/state"
}

// FilesConfig specifies paths for data files.
type FilesConfig struct {
	Agents string `json:"agents"` // default "/etc/clawgress/agents.json"
	Policy string `json:"policy"` // default "/etc/clawgress/policy.json"
	Quotas string `json:"quotas"` // default "/etc/clawgress/quotas.json"
	Audit  string `json:"audit"`  // default "/var/log/clawgress/audit.jsonl"
	SQLite string `json:"sqlite"` // default "" (empty = file-only mode)
}

// Defaults returns a Config with all default values.
func Defaults() Config {
	return Config{
		Gateway: GatewayConfig{
			Listen:       ":3128",
			ReadTimeout:  60,
			WriteTimeout: 0,
		},
		AdminAPI: AdminAPIConfig{
			Listen:   ":8080",
			UIPath:   "/ui/",
			StateDir: "/var/lib/clawgress/state",
		},
		Files: FilesConfig{
			Agents: "/etc/clawgress/agents.json",
			Policy: "/etc/clawgress/policy.json",
			Quotas: "/etc/clawgress/quotas.json",
			Audit:  "/var/log/clawgress/audit.jsonl",
		},
	}
}

// Load reads and validates a config file. Returns defaults if the file doesn't exist.
func Load(path string) (Config, error) {
	cfg := Defaults()
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}

	// Strict decoding: reject unknown fields.
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}

	if err := validate(cfg); err != nil {
		return cfg, fmt.Errorf("validate config %s: %w", path, err)
	}

	return cfg, nil
}

func validate(cfg Config) error {
	if cfg.Gateway.Listen == "" {
		return fmt.Errorf("gateway.listen must not be empty")
	}
	if cfg.AdminAPI.Listen == "" {
		return fmt.Errorf("admin_api.listen must not be empty")
	}
	if cfg.Gateway.ReadTimeout < 0 {
		return fmt.Errorf("gateway.read_timeout_s must be >= 0")
	}
	if cfg.Gateway.WriteTimeout < 0 {
		return fmt.Errorf("gateway.write_timeout_s must be >= 0")
	}
	if cfg.Files.Agents == "" {
		return fmt.Errorf("files.agents must not be empty")
	}
	if cfg.Files.Policy == "" {
		return fmt.Errorf("files.policy must not be empty")
	}
	if cfg.Files.Audit == "" {
		return fmt.Errorf("files.audit must not be empty")
	}
	return nil
}
