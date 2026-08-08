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

func TestResolveHonorsPathOrder(t *testing.T) {
	ResetCache()
	dir := t.TempDir()
	// Only espeak exists -> TTS resolves to espeak.
	fakeBin(t, dir, "espeak")
	t.Setenv("PATH", dir)

	program, args, ok := Resolve(ModeTTS)
	if !ok {
		t.Fatal("expected TTS provider")
	}
	if program != "espeak" {
		t.Errorf("program = %q, want espeak", program)
	}
	if len(args) != 1 || args[0] != "{TEXT}" {
		t.Errorf("args = %v", args)
	}
}

func TestResolvePrefersFirstInChain(t *testing.T) {
	ResetCache()
	dir := t.TempDir()
	fakeBin(t, dir, "espeak-ng")
	fakeBin(t, dir, "espeak")
	t.Setenv("PATH", dir)

	program, _, ok := Resolve(ModeTTS)
	if !ok || program != "espeak-ng" {
		t.Fatalf("expected espeak-ng, got %q ok=%v", program, ok)
	}
}

func TestResolveNoneAvailable(t *testing.T) {
	ResetCache()
	dir := t.TempDir()
	t.Setenv("PATH", dir) // empty PATH (no tools)

	if _, _, ok := Resolve(ModeTTS); ok {
		t.Fatal("expected no TTS provider")
	}
	// Second call uses cache and still reports none.
	if _, _, ok := Resolve(ModeTTS); ok {
		t.Fatal("cached result should also be none")
	}
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
