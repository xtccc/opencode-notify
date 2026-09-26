package plugin

import (
	"encoding/json"
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
}

func TestRenderSessionTitlePresent(t *testing.T) {
	out, err := Render("/usr/local/bin/opencode-notify")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"session_title",
		"fetchSessionContext",
		"projectDisplayName",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("session title %q missing from template", want)
		}
	}
	if strings.Contains(out, "location', 'project', 'id'") {
		t.Error("template must not use hex project.id as project_name")
	}
}

func TestRenderLeaderElectionPresent(t *testing.T) {
	out, err := Render("/usr/local/bin/opencode-notify")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"plugin-leader.lock",
		"tryAcquireLeader",
		"refreshLeaderHeartbeat",
		"releaseLeader",
		"LEADER_STALE_MS",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("leader election %q missing from template", want)
		}
	}
}

func TestRenderV2PluginShape(t *testing.T) {
	out, err := Render("/usr/local/bin/opencode-notify")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "export default") {
		t.Error("template must default-export a v2 plugin definition")
	}
	if !strings.Contains(out, "id: 'opencode-notify'") {
		t.Error("template must declare the plugin id")
	}
	if !strings.Contains(out, "async setup(ctx)") {
		t.Error("template must use the v2 Promise setup function")
	}
	if strings.Contains(out, "from '@opencode/plugin'") || strings.Contains(out, `from "@opencode/plugin"`) {
		t.Error("template must not import @opencode/plugin (unresolvable for local plugin dirs)")
	}
	if strings.Contains(out, "OpenCodeNotifyPlugin") {
		t.Error("v1 named export must be gone")
	}
}

func TestRenderV2EventNames(t *testing.T) {
	out, err := Render("/usr/local/bin/opencode-notify")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"session.execution.succeeded",
		"session.execution.failed",
		"session.execution.interrupted",
		"form.created",
		"permission.asked",
		"session.idle",
	} {
		if !strings.Contains(out, name) {
			t.Errorf("template missing event %q", name)
		}
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
	if strings.Contains(out, `""`) {
		t.Error("fallback must not bake an empty executable path")
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

func TestRenderManifest(t *testing.T) {
	out, err := RenderManifest()
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal([]byte(out), &manifest); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	if manifest["name"] != PackageName {
		t.Errorf("manifest name = %v", manifest["name"])
	}
	if manifest["type"] != "module" {
		t.Errorf("manifest type = %v, want module", manifest["type"])
	}
	exports, ok := manifest["exports"].(map[string]any)
	if !ok || exports["."] != "./index.js" {
		t.Errorf("manifest exports = %v", manifest["exports"])
	}
}
