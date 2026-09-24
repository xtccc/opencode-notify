package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"opencode-notify/internal/config"
	"opencode-notify/internal/plugin"
)

func setupEnv(t *testing.T) {
	t.Helper()
	t.Setenv("OPENCODE_CONFIG_DIR", filepath.Join(t.TempDir(), "opencode"))
}

func TestInstallWritesPluginDirectory(t *testing.T) {
	setupEnv(t)
	path, err := Install("/opt/opencode-notify/opencode-notify")
	if err != nil {
		t.Fatal(err)
	}
	if path != config.OpenCodePluginDir() {
		t.Errorf("path = %q", path)
	}
	index, err := os.ReadFile(filepath.Join(path, plugin.IndexFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), config.PluginMarker) {
		t.Error("installed entrypoint missing marker")
	}
	if !strings.Contains(string(index), "/opt/opencode-notify/opencode-notify") {
		t.Error("installed entrypoint missing exe path")
	}
	manifest, err := os.ReadFile(filepath.Join(path, plugin.ManifestFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), "./"+plugin.IndexFileName) {
		t.Error("manifest missing index.js export")
	}
}

func TestUninstallRemovesOwnedDirectory(t *testing.T) {
	setupEnv(t)
	if _, err := Install("/tmp/opencode-notify"); err != nil {
		t.Fatal(err)
	}
	removed, err := Uninstall()
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Error("expected directory to be removed")
	}
	if _, err := os.Stat(config.OpenCodePluginDir()); !os.IsNotExist(err) {
		t.Error("plugin directory still exists after uninstall")
	}
	// second uninstall is a no-op
	removed, err = Uninstall()
	if err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Error("second uninstall should remove nothing")
	}
}

func TestUninstallRemovesLegacyFile(t *testing.T) {
	setupEnv(t)
	legacy := config.LegacyOpenCodePluginPath()
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("// opencode-notify:plugin\nv1"), 0o644); err != nil {
		t.Fatal(err)
	}
	removed, err := Uninstall()
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Error("expected legacy file to be removed")
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Error("legacy plugin file still exists after uninstall")
	}
}

func TestUninstallLeavesForeignDirectory(t *testing.T) {
	setupEnv(t)
	dir := config.OpenCodePluginDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, plugin.IndexFileName), []byte("// some other plugin"), 0o644); err != nil {
		t.Fatal(err)
	}
	removed, err := Uninstall()
	if err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Error("foreign directory must not be removed")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Error("foreign directory should still exist")
	}
}

func TestStatusReflectsInstallation(t *testing.T) {
	setupEnv(t)
	st := Status()
	if st.Installed {
		t.Error("should not be installed initially")
	}
	if _, err := Install("/tmp/opencode-notify"); err != nil {
		t.Fatal(err)
	}
	st = Status()
	if !st.Installed {
		t.Error("should be installed after Install")
	}
	if st.PluginPath != config.OpenCodePluginDir() {
		t.Errorf("pluginPath = %q", st.PluginPath)
	}
}
