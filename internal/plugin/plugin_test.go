package plugin

import (
	"strings"
	"testing"

	"opencode-notify/internal/config"
)

func TestRenderContainsMarker(t *testing.T) {
	out, err := Render("/usr/local/bin/opencode-notify")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, config.PluginMarker) {
		t.Error("missing plugin marker")
	}
}

func TestRenderBakesExecutable(t *testing.T) {
	out, err := Render("/usr/local/bin/opencode-notify")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"/usr/local/bin/opencode-notify"`) {
		t.Error("exe path not baked into NOTIFY_CMD")
	}
	if !strings.Contains(out, `"notify"`) ||
		!strings.Contains(out, `"--source"`) ||
		!strings.Contains(out, `"--from-hook"`) {
		t.Error("notify argv incomplete")
	}
	if strings.Contains(out, `"--force"`) {
		t.Error("plugin command must not disable dedupe with --force")
	}
}

func TestRenderCoalescingPresent(t *testing.T) {
	out, err := Render("/usr/local/bin/opencode-notify")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "COALESCE_MS") {
		t.Error("per-session coalescing constant missing from template")
	}
	if strings.Contains(out, "lastEventKey") {
		t.Error("old exact-key dedupe code should have been removed")
	}
}

func TestRenderDefaultFallback(t *testing.T) {
	out, err := Render("")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"opencode-notify"`) {
		t.Error("fallback binary name missing")
	}
}

func TestRenderNoPlaceholderLeft(t *testing.T) {
	out, err := Render("/tmp/opencode-notify")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "__NOTIFY_CMD_JSON__") {
		t.Error("placeholder not substituted")
	}
}

func TestRenderEnvOverrideSupport(t *testing.T) {
	out, err := Render("/usr/local/bin/opencode-notify")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "OPENCODE_NOTIFY_BIN") {
		t.Error("env override hook missing from template")
	}
}
