package sound

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// fakeBin creates an executable named `name` in dir (shell script on unix).
func fakeBin(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	var script string
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nexit /b 0\r\n"
	} else {
		script = "#!/bin/sh\nexit 0\n"
	}
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake bin %s: %v", name, err)
	}
	return path
}

func TestResolvePlayChain(t *testing.T) {
	ResetCache()
	dir := t.TempDir()
	fakeBin(t, dir, "aplay") // skips paplay/pw-play since absent
	t.Setenv("PATH", dir)

	program, args, ok := Resolve(ModePlay)
	if !ok {
		t.Fatal("expected play provider")
	}
	if program != "aplay" {
		t.Errorf("program = %q, want aplay", program)
	}
	if len(args) != 1 || args[0] != "{FILE}" {
		t.Errorf("args = %v", args)
	}
}

func TestResolveBeepWithBellFile(t *testing.T) {
	ResetCache()
	dir := t.TempDir()
	fakeBin(t, dir, "canberra-gtk-play")
	t.Setenv("PATH", dir)

	program, args, ok := Resolve(ModeBeep)
	if !ok || program != "canberra-gtk-play" {
		t.Fatalf("expected canberra-gtk-play, got %q ok=%v", program, ok)
	}
	if len(args) != 2 || args[0] != "-i" || args[1] != "bell" {
		t.Errorf("args = %v", args)
	}
}
