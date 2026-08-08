package sound

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"opencode-notify/internal/config"
)

func TestPlayDisabled(t *testing.T) {
	cfg := config.SoundConfig{Enabled: false}
	res := Play(context.Background(), cfg, "x")
	if res.OK {
		t.Fatal("disabled channel must fail")
	}
}

func TestPlayOverrideTrue(t *testing.T) {
	cfg := config.SoundConfig{Enabled: true, OverrideCommand: "/bin/true $TEXT"}
	res := Play(context.Background(), cfg, "Hello")
	if !res.OK {
		t.Fatalf("override should succeed: %+v", res)
	}
	if res.Mode != "override" {
		t.Errorf("mode = %q", res.Mode)
	}
}

func TestPlayOverrideFalse(t *testing.T) {
	cfg := config.SoundConfig{Enabled: true, OverrideCommand: "/bin/false"}
	res := Play(context.Background(), cfg, "x")
	if res.OK {
		t.Fatal("override /bin/false should fail")
	}
}

func TestPlayAudioFileMissing(t *testing.T) {
	cfg := config.SoundConfig{Enabled: true, TTS: false, AudioPath: "/nonexistent/file.wav", FallbackBeep: false}
	res := Play(context.Background(), cfg, "x")
	if res.OK {
		t.Fatal("missing file should fail")
	}
}

func TestPlayAudioFileWithFakePlayer(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "ping.wav")
	if err := os.WriteFile(file, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	// fake paplay that records argv
	script := "#!/bin/sh\necho \"$@\" > " + filepath.Join(dir, "called") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "paplay"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	ResetCache()

	cfg := config.SoundConfig{Enabled: true, TTS: false, AudioPath: file, FallbackBeep: false}
	res := Play(context.Background(), cfg, "x")
	if !res.OK {
		t.Fatalf("play should succeed: %+v", res)
	}
	if res.Provider != "paplay" {
		t.Errorf("provider = %q", res.Provider)
	}
	data, err := os.ReadFile(filepath.Join(dir, "called"))
	if err != nil {
		t.Fatalf("read called: %v", err)
	}
	if string(data) != file+"\n" {
		t.Errorf("player argv = %q, want %q", string(data), file)
	}
}

func TestPlayTTSUsesStaticText(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "spoken")
	script := "#!/bin/sh\nprintf '%s' \"$*\" > " + record + "\n"
	if err := os.WriteFile(filepath.Join(dir, "espeak"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	ResetCache()

	cfg := config.SoundConfig{Enabled: true, TTS: true, StaticText: "任务完成", FallbackBeep: false}
	res := Play(context.Background(), cfg, "ignored")
	if !res.OK {
		t.Fatalf("tts should succeed: %+v", res)
	}
	data, _ := os.ReadFile(record)
	if string(data) != "任务完成" {
		t.Errorf("spoken text = %q, want 任务完成", string(data))
	}
}

func TestPlayNoToolNoBeep(t *testing.T) {
	ResetCache()
	dir := t.TempDir()
	t.Setenv("PATH", dir) // no tools at all
	cfg := config.SoundConfig{Enabled: true, TTS: true, FallbackBeep: false}
	res := Play(context.Background(), cfg, "x")
	if res.OK {
		t.Fatal("no tool and no fallback must fail")
	}
	if res.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestExpandPlaceholders(t *testing.T) {
	if got := expandTTS([]string{"{TEXT}"}, "hello world"); len(got) != 1 || got[0] != "hello world" {
		t.Errorf("expandTTS = %v", got)
	}
	if got := expandFile([]string{"-nodisp", "{FILE}"}, "/tmp/a.wav"); got[1] != "/tmp/a.wav" {
		t.Errorf("expandFile = %v", got)
	}
}
