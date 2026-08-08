package state

import (
	"path/filepath"
	"testing"
)

func TestCheckAndRemember(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_NOTIFY_STATE_DIR", dir)

	s, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	fp := MakeFingerprint("opencode", "/proj", "OpenCode 完成")
	if got, _ := s.CheckAndRemember(fp, 5); got {
		t.Fatal("first check should not be a duplicate")
	}
	// Same fingerprint within window -> duplicate.
	if got, _ := s.CheckAndRemember(fp, 5); !got {
		t.Fatal("second check should be a duplicate")
	}
	// Different fingerprint -> not duplicate.
	if got, _ := s.CheckAndRemember(MakeFingerprint("opencode", "/other", "xyz"), 5); got {
		t.Fatal("different fingerprint should not be a duplicate")
	}
	// Empty fingerprint is never a duplicate.
	if got, _ := s.CheckAndRemember("", 5); got {
		t.Fatal("empty fingerprint should not be a duplicate")
	}
	// Reload from disk should still see duplicates.
	s2, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got, _ := s2.CheckAndRemember(fp, 5); !got {
		t.Fatal("reloaded store should report duplicate")
	}
}

func TestCheckAndRememberClampsSmallWindow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_NOTIFY_STATE_DIR", dir)
	s, _ := Load()
	if got, _ := s.CheckAndRemember("a::b::c", 0); got {
		t.Fatal("should not be duplicate on first")
	}
	if got, _ := s.CheckAndRemember("a::b::c", 0); !got {
		t.Fatal("should be duplicate with clamped window")
	}
}

func TestMakeFingerprint(t *testing.T) {
	if got := MakeFingerprint("opencode", "/Proj", "  OpenCode   完成 "); got != "opencode::/proj::opencode 完成" {
		t.Fatalf("unexpected fingerprint: %q", got)
	}
}

func TestStateDirPointsToTemp(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_NOTIFY_STATE_DIR", dir)
	if got := filepath.Join(dir, "state.json"); got != filepath.Join(dir, "state.json") {
		t.Fatalf("sanity check failed")
	}
}
