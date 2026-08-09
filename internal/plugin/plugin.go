// Package plugin renders the embedded opencode plugin JS with the
// notify binary path baked in.
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

// Render substitutes the NOTIFY_CMD placeholder with the JSON-encoded
// command and verifies the ownership marker is present. exePath is the
// absolute path to the opencode-notify binary ("" disables the baked
// path, relying on OPENCODE_NOTIFY_BIN / PATH).
func Render(exePath string) (string, error) {
	// No --force: plugin events must go through the normal dedupe/duration
	// pipeline so near-simultaneous bursts stay single-send.
	cmd := []string{"notify", "--source", "opencode", "--from-hook"}
	if exePath != "" {
		cmd = append([]string{exePath}, cmd...)
	} else {
		cmd = append([]string{"opencode-notify"}, cmd...)
	}
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
