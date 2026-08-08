package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"opencode-notify/internal/config"
)

func setupEnv(t *testing.T) {
	t.Helper()
	t.Setenv("OPENCODE_CONFIG_DIR", filepath.Join(t.TempDir(), "opencode"))
}

func TestInstallWritesMarkerFile(t *testing.T) {
	setupEnv(t)
	path, err := Install("/opt/opencode-notify/opencode-notify")
	if err != nil {
		t.Fatal(err)
	}
	if path != config.OpenCodePluginPath() {
		t.Errorf("path = %q", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), config.PluginMarker) {
		t.Error("installed file missing marker")
	}
	if !strings.Contains(string(data), "/opt/opencode-notify/opencode-notify") {
		t.Error("installed file missing exe path")
	}
}

func TestUninstallRemovesOwnedFile(t *testing.T) {
	setupEnv(t)
	if _, err := Install("/tmp/opencode-notify"); err != nil {
		t.Fatal(err)
	}
	removed, err := Uninstall()
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Error("expected file to be removed")
	}
	if _, err := os.Stat(config.OpenCodePluginPath()); !os.IsNotExist(err) {
		t.Error("plugin file still exists after uninstall")
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

func TestUninstallLeavesForeignFile(t *testing.T) {
	setupEnv(t)
	path := config.OpenCodePluginPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("// some other plugin"), 0o644); err != nil {
		t.Fatal(err)
	}
	removed, err := Uninstall()
	if err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Error("foreign file must not be removed")
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
}
