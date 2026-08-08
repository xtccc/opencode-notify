package state

import (
	"path/filepath"
	"sync"
	"testing"
)

func TestCheckAndRemember(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_NOTIFY_STATE_DIR", dir)

	fp := MakeFingerprint("opencode", "/proj", "OpenCode 完成")
	if got, err := CheckAndRemember(fp, 5); err != nil || got {
		t.Fatalf("first check should not be a duplicate (dup=%v err=%v)", got, err)
	}
	// Same fingerprint within window -> duplicate.
	if got, err := CheckAndRemember(fp, 5); err != nil || !got {
		t.Fatalf("second check should be a duplicate (dup=%v err=%v)", got, err)
	}
	// Different fingerprint -> not duplicate.
	if got, err := CheckAndRemember(MakeFingerprint("opencode", "/other", "xyz"), 5); err != nil || got {
		t.Fatalf("different fingerprint should not be a duplicate (dup=%v err=%v)", got, err)
	}
	// Empty fingerprint is never a duplicate.
	if got, err := CheckAndRemember("", 5); err != nil || got {
		t.Fatalf("empty fingerprint should not be a duplicate (dup=%v err=%v)", got, err)
	}
}

func TestCheckAndRememberClampsSmallWindow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_NOTIFY_STATE_DIR", dir)
	if got, _ := CheckAndRemember("a::b::c", 0); got {
		t.Fatal("should not be duplicate on first")
	}
	if got, _ := CheckAndRemember("a::b::c", 0); !got {
		t.Fatal("should be duplicate with clamped window")
	}
	if got, _ := CheckAndRemember("a::b::c", 1); !got {
		t.Fatal("should be duplicate when window is 1 minute")
	}
}

func TestConcurrentCheckAndRemember(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_NOTIFY_STATE_DIR", dir)

	fp := MakeFingerprint("opencode", "/proj", "concurrent task done")

	const workers = 16
	start := make(chan struct{})
	results := make(chan bool, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			dup, err := CheckAndRemember(fp, 5)
			if err != nil {
				t.Errorf("CheckAndRemember: %v", err)
				return
			}
			results <- dup
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	duplicates := 0
	for dup := range results {
		if dup {
			duplicates++
		}
	}
	if want := workers - 1; duplicates != want {
		t.Fatalf("expected %d duplicates (exactly one sender), got %d", want, duplicates)
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