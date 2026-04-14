package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConfigMatrixDefaults verifies defaults work with no config file.
func TestConfigMatrixDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Gateway.Listen != ":3128" {
		t.Fatal("default gateway listen wrong")
	}
	if cfg.Files.SQLite != "" {
		t.Fatal("SQLite should be empty by default (file-only mode)")
	}
}

// TestConfigMatrixWithSQLite verifies SQLite path is accepted.
func TestConfigMatrixWithSQLite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"files":{"sqlite":"/var/lib/clawgress/clawgress.db"}}`), 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Files.SQLite != "/var/lib/clawgress/clawgress.db" {
		t.Fatalf("SQLite path wrong: %s", cfg.Files.SQLite)
	}
	// Other defaults still set.
	if cfg.Gateway.Listen != ":3128" {
		t.Fatal("defaults overwritten")
	}
}

// TestConfigMatrixCustomPorts verifies custom listen addresses.
func TestConfigMatrixCustomPorts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"gateway":{"listen":":9128"},"admin_api":{"listen":":9080"}}`), 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Gateway.Listen != ":9128" {
		t.Fatalf("gateway listen: %s", cfg.Gateway.Listen)
	}
	if cfg.AdminAPI.Listen != ":9080" {
		t.Fatalf("admin listen: %s", cfg.AdminAPI.Listen)
	}
}

// TestConfigMatrixAllFields verifies a full config file.
func TestConfigMatrixAllFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{
		"gateway": {"listen": ":3128", "read_timeout_s": 30, "write_timeout_s": 0},
		"admin_api": {"listen": ":8080", "ui_path": "/ui/", "state_dir": "/tmp/state"},
		"files": {
			"agents": "/tmp/agents.json",
			"policy": "/tmp/policy.json",
			"quotas": "/tmp/quotas.json",
			"audit": "/tmp/audit.jsonl",
			"sqlite": "/tmp/test.db"
		}
	}`), 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Files.Agents != "/tmp/agents.json" {
		t.Fatal("agents path")
	}
	if cfg.Files.SQLite != "/tmp/test.db" {
		t.Fatal("sqlite path")
	}
	if cfg.Gateway.ReadTimeout != 30 {
		t.Fatal("read timeout")
	}
}

// TestConfigMatrixRejectsInvalid tests various invalid configs.
func TestConfigMatrixRejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{"empty gateway listen", `{"gateway":{"listen":""}}`},
		{"empty admin listen", `{"admin_api":{"listen":""}}`},
		{"empty agents path", `{"files":{"agents":""}}`},
		{"empty policy path", `{"files":{"policy":""}}`},
		{"empty audit path", `{"files":{"audit":""}}`},
		{"negative read timeout", `{"gateway":{"read_timeout_s":-1}}`},
	}

	for _, tc := range cases {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.json")
		os.WriteFile(path, []byte(tc.json), 0o644)

		_, err := Load(path)
		if err == nil {
			t.Fatalf("%s: expected validation error", tc.name)
		}
	}
}
