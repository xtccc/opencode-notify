// Package hook manages the generated opencode plugin directory:
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

// StatusInfo describes the current plugin/installation state.
type StatusInfo struct {
	Installed      bool   `json:"installed"`
	PluginPath     string `json:"pluginPath"`
	OpencodeConfig string `json:"opencodeConfigDir"`
	SettingsPath   string `json:"settingsPath"`
	Executable     string `json:"executable,omitempty"`
}

// Install renders the plugin directory (package.json + index.js) with the
// given binary path and writes it under the opencode plugins directory.
// Returns the directory path.
func Install(exePath string) (string, error) {
	index, err := plugin.Render(exePath)
	if err != nil {
		return "", err
	}
	manifest, err := plugin.RenderManifest()
	if err != nil {
		return "", err
	}
	dir := config.OpenCodePluginDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, plugin.ManifestFileName), []byte(manifest), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, plugin.IndexFileName), []byte(index), 0o644); err != nil {
		return "", err
	}
	return dir, nil
}

// Uninstall removes the plugin directory only when its entrypoint carries our
// marker. Returns whether files were actually removed. Legacy single-file
// installs (plugins/opencode-notify.js) are cleaned up as well.
func Uninstall() (bool, error) {
	removed := false

	if legacy := config.LegacyOpenCodePluginPath(); legacy != "" {
		ok, err := removeOwnedFile(legacy)
		if err != nil {
			return removed, err
		}
		removed = removed || ok
	}

	dir := config.OpenCodePluginDir()
	data, err := os.ReadFile(filepath.Join(dir, plugin.IndexFileName))
	if errors.Is(err, os.ErrNotExist) {
		return removed, nil
	}
	if err != nil {
		return removed, err
	}
	if !strings.Contains(string(data), config.PluginMarker) {
		return removed, nil
	}
	if err := os.RemoveAll(dir); err != nil {
		return removed, err
	}
	return true, nil
}

// removeOwnedFile deletes a single file when it carries our marker.
func removeOwnedFile(path string) (bool, error) {
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

// Status reports whether our plugin directory is present at the expected
// location. A legacy single-file install is reported as installed too.
func Status() StatusInfo {
	dir := config.OpenCodePluginDir()
	installed := false
	if data, err := os.ReadFile(filepath.Join(dir, plugin.IndexFileName)); err == nil {
		installed = strings.Contains(string(data), config.PluginMarker)
	}
	if !installed {
		if legacy := config.LegacyOpenCodePluginPath(); legacy != "" {
			if data, err := os.ReadFile(legacy); err == nil {
				installed = strings.Contains(string(data), config.PluginMarker)
			}
		}
	}
	return StatusInfo{
		Installed:      installed,
		PluginPath:     dir,
		OpencodeConfig: config.OpenCodeConfigDir(),
		SettingsPath:   config.SettingsPath(),
	}
}
