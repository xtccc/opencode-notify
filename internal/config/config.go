package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// CurrentVersion is the settings schema version this binary writes/reads.
const CurrentVersion = 1

// PriorityMap holds per-kind Gotify priorities.
type PriorityMap struct {
	Complete int `json:"complete"`
	Error    int `json:"error"`
	Question int `json:"question"`
}

// DefaultPriority returns the built-in priority map.
func DefaultPriority() PriorityMap {
	return PriorityMap{Complete: 5, Error: 10, Question: 6}
}

// GotifyConfig holds the Gotify channel settings.
type GotifyConfig struct {
	URL       string      `json:"url"`
	AppToken  string      `json:"appToken"`
	TimeoutMs int         `json:"timeoutMs"`
	Priority  PriorityMap `json:"priority"`
}

// OpenCodeConfig holds per-source settings for opencode.
type OpenCodeConfig struct {
	Enabled            bool `json:"enabled"`
	MinDurationMinutes int  `json:"minDurationMinutes"`
}

// SoundConfig holds the sound channel settings.
type SoundConfig struct {
	Enabled         bool   `json:"enabled"`
	TTS             bool   `json:"tts"`
	AudioPath       string `json:"audioPath"`
	StaticText      string `json:"staticText"`
	FallbackBeep    bool   `json:"fallbackBeep"`
	OverrideCommand string `json:"overrideCommand"`
	MimoAPIKey      string `json:"mimoApiKey"`
	TTSVoice        string `json:"ttsVoice"`
}

// DedupeConfig holds notification dedupe settings.
type DedupeConfig struct {
	Enabled       bool `json:"enabled"`
	WindowMinutes int  `json:"windowMinutes"`
}

// UIConfig holds UI/language settings (minimal).
type UIConfig struct {
	Language string `json:"language"`
}

// Config is the full settings schema (version 1).
type Config struct {
	Version  int            `json:"version"`
	Gotify   GotifyConfig   `json:"gotify"`
	OpenCode OpenCodeConfig `json:"opencode"`
	Sound    SoundConfig    `json:"sound"`
	Dedupe   DedupeConfig   `json:"dedupe"`
	UI       UIConfig       `json:"ui"`
}

// Default returns the built-in default config.
func Default() Config {
	return Config{
		Version: CurrentVersion,
		Gotify: GotifyConfig{
			TimeoutMs: 10000,
			Priority:  DefaultPriority(),
		},
		OpenCode: OpenCodeConfig{
			Enabled: true,
		},
		Sound: SoundConfig{
			Enabled:      true,
			TTS:          true,
			FallbackBeep: true,
			TTSVoice:     "Chloe",
		},
		Dedupe: DedupeConfig{
			Enabled:       true,
			WindowMinutes: 5,
		},
		UI: UIConfig{Language: "zh-CN"},
	}
}

// loadRaw reads settings.json if present. Missing file returns (nil, nil).
func loadRaw() (*Config, map[string]struct{}, error) {
	data, err := os.ReadFile(SettingsPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil, nil
	}
	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, nil, fmt.Errorf("settings.json: %w", err)
	}
	present := map[string]struct{}{}
	var sections map[string]json.RawMessage
	if err := json.Unmarshal(data, &sections); err == nil {
		for section := range sections {
			present[section] = struct{}{}
		}
	}
	return cfg, present, nil
}

// normalize fills zero/empty fields from defaults and clamps ranges.
func (c *Config) normalize(present map[string]struct{}) {
	d := Default()

	if c.Version <= 0 {
		c.Version = CurrentVersion
	}
	if _, ok := present["gotify"]; !ok {
		c.Gotify = d.Gotify
	}
	if c.Gotify.TimeoutMs <= 0 {
		c.Gotify.TimeoutMs = d.Gotify.TimeoutMs
	}
	if c.Gotify.Priority.Complete <= 0 {
		c.Gotify.Priority.Complete = d.Gotify.Priority.Complete
	}
	if c.Gotify.Priority.Error <= 0 {
		c.Gotify.Priority.Error = d.Gotify.Priority.Error
	}
	if c.Gotify.Priority.Question <= 0 {
		c.Gotify.Priority.Question = d.Gotify.Priority.Question
	}
	if _, ok := present["opencode"]; !ok {
		c.OpenCode = d.OpenCode
	} else if c.OpenCode.MinDurationMinutes < 0 {
		c.OpenCode.MinDurationMinutes = 0
	}
	if _, ok := present["sound"]; !ok {
		c.Sound = d.Sound
	}
	if strings.TrimSpace(c.Sound.TTSVoice) == "" {
		c.Sound.TTSVoice = d.Sound.TTSVoice
	}
	if _, ok := present["dedupe"]; !ok {
		c.Dedupe = d.Dedupe
	} else if c.Dedupe.WindowMinutes <= 0 {
		c.Dedupe.WindowMinutes = d.Dedupe.WindowMinutes
	}
	if strings.TrimSpace(c.UI.Language) == "" {
		c.UI.Language = d.UI.Language
	}
}

// applyEnvOverrides applies the documented env var overrides.
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("OPENCODE_NOTIFY_GOTIFY_URL"); strings.TrimSpace(v) != "" {
		c.Gotify.URL = strings.TrimSpace(v)
	}
	if v := os.Getenv("OPENCODE_NOTIFY_GOTIFY_TOKEN"); strings.TrimSpace(v) != "" {
		c.Gotify.AppToken = strings.TrimSpace(v)
	}
	if v := os.Getenv("OPENCODE_NOTIFY_MIMO_API_KEY"); strings.TrimSpace(v) != "" {
		c.Sound.MimoAPIKey = strings.TrimSpace(v)
	}
}

// Load reads, normalizes and applies env overrides. A missing or empty
// settings.json yields the default config without error. A malformed JSON
// returns an error so callers can surface it.
func Load() (Config, error) {
	raw, present, err := loadRaw()
	if err != nil {
		return Config{}, err
	}
	cfg := Default()
	if raw != nil {
		cfg = *raw
	}
	if present == nil {
		present = map[string]struct{}{}
	}
	cfg.normalize(present)
	cfg.applyEnvOverrides()
	return cfg, nil
}

// Save writes the config to settings.json (creates the dir if needed).
// All sections are considered present so their values are preserved,
// while zero/empty fields still get clamped to defaults.
func Save(cfg Config) error {
	if err := EnsureConfigDir(); err != nil {
		return err
	}
	present := map[string]struct{}{
		"gotify":   {},
		"opencode": {},
		"sound":    {},
		"dedupe":   {},
		"ui":       {},
	}
	cfg.normalize(present)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(SettingsPath(), append(data, '\n'), 0o644)
}
