// Package hook manages the generated opencode plugin file:
// install (write), uninstall (remove by marker), status.
package hook

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"opencode-notify/internal/config"
	"opencode-notify/internal/plugin"
)

// Status describes the current plugin/installation state.
type StatusInfo struct {
	Installed      bool   `json:"installed"`
	PluginPath     string `json:"pluginPath"`
	OpencodeConfig string `json:"opencodeConfigDir"`
	SettingsPath   string `json:"settingsPath"`
	Executable     string `json:"executable,omitempty"`
}

// Install renders the plugin with the given binary path and writes it to
// the opencode plugins directory. Returns the written path.
func Install(exePath string) (string, error) {
	js, err := plugin.Render(exePath)
	if err != nil {
		return "", err
	}
	path := config.OpenCodePluginPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(js), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// Uninstall removes the plugin file only when it carries our marker.
// Returns whether a file was actually removed.
func Uninstall() (bool, error) {
	path := config.OpenCodePluginPath()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !strings.Contains(string(data), config.PluginMarker) {
		return false, nil
	}
	if err := os.Remove(path); err != nil {
		return false, err
	}
	return true, nil
}

// Status reports whether our plugin is present at the expected location.
func Status() StatusInfo {
	path := config.OpenCodePluginPath()
	installed := false
	if data, err := os.ReadFile(path); err == nil {
		installed = strings.Contains(string(data), config.PluginMarker)
	}
	return StatusInfo{
		Installed:      installed,
		PluginPath:     path,
		OpencodeConfig: config.OpenCodeConfigDir(),
		SettingsPath:   config.SettingsPath(),
	}
}
