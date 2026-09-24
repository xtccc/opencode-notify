// Package plugin renders the embedded opencode v2 plugin source (index.js)
// and its manifest (package.json) with the notify binary path baked in.
package plugin

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"opencode-notify/internal/config"
)

//go:embed template.js
var templateSource string

// PackageName is the npm-style name of the generated plugin package.
const PackageName = "opencode-notify"

// IndexFileName is the plugin entrypoint inside the plugin directory.
const IndexFileName = "index.js"

// ManifestFileName is the package manifest inside the plugin directory.
const ManifestFileName = "package.json"

// Render substitutes the NOTIFY_CMD placeholder with the JSON-encoded command
// and verifies the ownership marker is present. exePath is the absolute path
// to the opencode-notify binary ("" disables the baked path, relying on
// OPENCODE_NOTIFY_BIN / PATH).
func Render(exePath string) (string, error) {
	// No --force: plugin events must go through the normal dedupe/duration
	// pipeline so near-simultaneous bursts stay single-send.
	binary := exePath
	if binary == "" {
		binary = "opencode-notify"
	}
	cmd := []string{binary, "notify", "--source", "opencode", "--from-hook"}
	cmdJSON, err := json.Marshal(cmd)
	if err != nil {
		return "", fmt.Errorf("plugin: marshal notify cmd: %w", err)
	}

	out := strings.ReplaceAll(templateSource, "__NOTIFY_CMD_JSON__", string(cmdJSON))
	if !strings.Contains(out, config.PluginMarker) {
		return "", fmt.Errorf("plugin: template missing marker %q", config.PluginMarker)
	}
	if strings.Contains(out, "__NOTIFY_CMD_JSON__") {
		return "", fmt.Errorf("plugin: NOTIFY_CMD placeholder not substituted")
	}
	return out, nil
}

// RenderManifest returns the package.json written next to index.js. OpenCode v2
// discovers configured/auto-discovered plugin directories through
// Host.resolve, which resolves package.json exports.
func RenderManifest() (string, error) {
	manifest := map[string]any{
		"name":        PackageName,
		"version":     "1.0.0",
		"private":     true,
		"type":        "module",
		"description": "opencode-notify plugin (generated). Do not edit by hand.",
		"exports": map[string]any{
			".": "./" + IndexFileName,
		},
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("plugin: marshal manifest: %w", err)
	}
	return string(data) + "\n", nil
}
