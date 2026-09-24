package config

import (
	"os"
	"path/filepath"
)

// DefaultConfigDirName is the product-level directory name used under
// XDG config/state roots.
const DefaultConfigDirName = "opencode-notify"

// SettingsFileName is the JSON config file name inside the config dir.
const SettingsFileName = "settings.json"

// StateFileName is the dedupe state file name inside the state dir.
const StateFileName = "state.json"

// OpenCodePluginFileName is the legacy single-file plugin name inside the
// opencode plugins directory (v1 layout).
const OpenCodePluginFileName = "opencode-notify.js"

// OpenCodePluginDirName is the plugin directory name inside the opencode
// plugins directory (v2 layout).
const OpenCodePluginDirName = "opencode-notify"

// PluginMarker identifies plugin files we own so uninstall can remove
// them safely.
const PluginMarker = "opencode-notify:plugin"

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// homeDir returns the current user's home directory (may be empty).
func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return ""
}

// ConfigDir returns the settings directory. Resolution order:
//  1. OPENCODE_NOTIFY_CONFIG_DIR env
//  2. $XDG_CONFIG_HOME/opencode-notify
//  3. ~/.config/opencode-notify
func ConfigDir() string {
	if override := os.Getenv("OPENCODE_NOTIFY_CONFIG_DIR"); override != "" {
		return filepath.Clean(override)
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, DefaultConfigDirName)
	}
	return filepath.Join(homeDir(), ".config", DefaultConfigDirName)
}

// StateDir returns the dedupe state directory. Resolution order:
//  1. OPENCODE_NOTIFY_STATE_DIR env
//  2. $XDG_STATE_HOME/opencode-notify
//  3. ~/.local/state/opencode-notify
func StateDir() string {
	if override := os.Getenv("OPENCODE_NOTIFY_STATE_DIR"); override != "" {
		return filepath.Clean(override)
	}
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, DefaultConfigDirName)
	}
	return filepath.Join(homeDir(), ".local", "state", DefaultConfigDirName)
}

// SettingsPath returns the full path to settings.json.
func SettingsPath() string {
	return filepath.Join(ConfigDir(), SettingsFileName)
}

// StatePath returns the full path to state.json.
func StatePath() string {
	return filepath.Join(StateDir(), StateFileName)
}

// OpenCodeConfigDir returns the opencode config directory.
// Resolution order:
//  1. OPENCODE_CONFIG_DIR env
//  2. ~/.config/opencode
func OpenCodeConfigDir() string {
	if override := os.Getenv("OPENCODE_CONFIG_DIR"); override != "" {
		return filepath.Clean(override)
	}
	return filepath.Join(homeDir(), ".config", "opencode")
}

// OpenCodePluginsDir returns the opencode plugins directory.
func OpenCodePluginsDir() string {
	return filepath.Join(OpenCodeConfigDir(), "plugins")
}

// OpenCodePluginDir returns the plugin directory installed for OpenCode v2
// (plugins/opencode-notify/).
func OpenCodePluginDir() string {
	return filepath.Join(OpenCodePluginsDir(), OpenCodePluginDirName)
}

// LegacyOpenCodePluginPath returns the v1 single-file plugin path
// (plugins/opencode-notify.js), kept for uninstall/status of old installs.
func LegacyOpenCodePluginPath() string {
	return filepath.Join(OpenCodePluginsDir(), OpenCodePluginFileName)
}

// EnsureConfigDir creates the settings directory if missing.
func EnsureConfigDir() error {
	return os.MkdirAll(ConfigDir(), 0o755)
}

// EnsureStateDir creates the state directory if missing.
func EnsureStateDir() error {
	return os.MkdirAll(StateDir(), 0o755)
}
