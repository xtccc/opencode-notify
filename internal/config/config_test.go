package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsWhenMissing(t *testing.T) {
	t.Setenv("OPENCODE_NOTIFY_CONFIG_DIR", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.OpenCode.Enabled {
		t.Error("opencode.enabled should default true")
	}
	if !cfg.Sound.Enabled || !cfg.Sound.TTS || !cfg.Sound.FallbackBeep {
		t.Error("sound defaults wrong")
	}
	if cfg.Dedupe.WindowMinutes != 5 {
		t.Errorf("dedupe window = %d", cfg.Dedupe.WindowMinutes)
	}
	if cfg.Gotify.TimeoutMs != 10000 {
		t.Errorf("timeout = %d", cfg.Gotify.TimeoutMs)
	}
}

func TestLoadFromFileAndNormalize(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_NOTIFY_CONFIG_DIR", dir)
	cfg := Default()
	cfg.Gotify.URL = "https://example.com"
	cfg.Sound.Enabled = false
	cfg.Dedupe.WindowMinutes = 1
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Gotify.URL != "https://example.com" {
		t.Errorf("url = %q", loaded.Gotify.URL)
	}
	if loaded.Sound.Enabled {
		t.Error("sound should be disabled")
	}
	// env override wins
	t.Setenv("OPENCODE_NOTIFY_GOTIFY_URL", "https://env.example.com")
	loaded, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Gotify.URL != "https://env.example.com" {
		t.Errorf("env override url = %q", loaded.Gotify.URL)
	}
}

func TestLoadMalformedJSONReturnsError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_NOTIFY_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, SettingsFileName), []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected error for malformed json")
	}
}

func TestLoadPartialConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_NOTIFY_CONFIG_DIR", dir)
	// only set gotify.url; everything else must fall back to defaults
	partial := `{"gotify": {"url": "http://localhost:8080"}}`
	if err := os.WriteFile(filepath.Join(dir, SettingsFileName), []byte(partial), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Gotify.URL != "http://localhost:8080" {
		t.Errorf("url = %q", cfg.Gotify.URL)
	}
	if cfg.Gotify.TimeoutMs != 10000 {
		t.Errorf("timeout = %d", cfg.Gotify.TimeoutMs)
	}
	if cfg.Gotify.Priority.Complete != 5 || cfg.Gotify.Priority.Error != 10 {
		t.Errorf("priorities = %+v", cfg.Gotify.Priority)
	}
	if cfg.Dedupe.WindowMinutes != 5 {
		t.Errorf("dedupe window = %d", cfg.Dedupe.WindowMinutes)
	}
}

func TestSaveCreatesDirAndJSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_NOTIFY_CONFIG_DIR", filepath.Join(dir, "nested", "dir"))
	cfg := Default()
	cfg.Gotify.AppToken = "secret-token"
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(SettingsPath())
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed["gotify"]; !ok {
		t.Error("missing gotify section in saved config")
	}
}
