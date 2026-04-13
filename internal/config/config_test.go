package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.Gateway.Listen != ":3128" {
		t.Fatalf("want :3128, got %s", cfg.Gateway.Listen)
	}
	if cfg.AdminAPI.Listen != ":8080" {
		t.Fatalf("want :8080, got %s", cfg.AdminAPI.Listen)
	}
	if cfg.Files.Agents != "/etc/clawgress/agents.json" {
		t.Fatalf("want default agents path, got %s", cfg.Files.Agents)
	}
}

func TestLoadMissing(t *testing.T) {
	cfg, err := Load("/nonexistent/config.json")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Gateway.Listen != ":3128" {
		t.Fatal("missing file should return defaults")
	}
}

func TestLoadEmpty(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Gateway.Listen != ":3128" {
		t.Fatal("empty path should return defaults")
	}
}

func TestLoadValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"gateway":{"listen":":9999"}}`), 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Gateway.Listen != ":9999" {
		t.Fatalf("want :9999, got %s", cfg.Gateway.Listen)
	}
	// Defaults still set for unspecified fields.
	if cfg.AdminAPI.Listen != ":8080" {
		t.Fatalf("unspecified fields should keep defaults, got %s", cfg.AdminAPI.Listen)
	}
}

func TestLoadUnknownField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"bogus_field": true}`), 0o644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("unknown field should cause error")
	}
}

func TestValidationEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"gateway":{"listen":""}}`), 0o644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("empty listen should fail validation")
	}
}
