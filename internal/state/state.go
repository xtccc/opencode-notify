// Package state provides a small JSON-file-backed dedupe store.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"opencode-notify/internal/config"
)

const (
	maxEntries = 200
	minWindow  = int64(30 * 1000)
)

// RecentNotification is one entry in the dedupe ring.
type RecentNotification struct {
	Fingerprint string `json:"fingerprint"`
	Timestamp   int64  `json:"timestamp"`
}

// persistedState is the on-disk shape of the state file.
type persistedState struct {
	RecentNotifications []RecentNotification `json:"recentNotifications"`
}

// Store holds the persisted dedupe state.
type Store struct {
	mu     sync.Mutex
	path   string
	recent []RecentNotification
}

// lockPath returns the dedicated lock file next to the state file. It is
// never renamed, so flock on it stays valid across the atomic (rename
// based) state rewrites of the state file itself.
func lockPath() string {
	return config.StatePath() + ".lock"
}

// acquireLock takes an exclusive flock on the lock file. The returned
// release function unlocks and closes the file; flock is also released
// automatically by the kernel if the process exits.
func acquireLock() (release func(), err error) {
	path := lockPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

// CheckAndRemember atomically records a fingerprint under a cross-process
// lock and reports whether an equivalent notification was already sent
// within the window. It is safe when several processes (e.g. two plugin
// hooks firing near-simultaneously) race on the same state file.
func CheckAndRemember(fp string, windowMinutes int) (bool, error) {
	if strings.TrimSpace(fp) == "" {
		return false, nil
	}

	unlock, err := acquireLock()
	if err != nil {
		return false, err
	}
	defer unlock()

	store, err := Load()
	if err != nil {
		return false, err
	}
	return store.checkAndRemember(fp, windowMinutes)
}

// Load opens (or creates empty) the state file.
func Load() (*Store, error) {
	store := &Store{path: config.StatePath()}
	data, err := os.ReadFile(store.path)
	if err == nil {
		var persisted persistedState
		_ = json.Unmarshal(data, &persisted)
		store.recent = persisted.RecentNotifications
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if store.recent == nil {
		store.recent = []RecentNotification{}
	}
	return store, nil
}

// Normalize normalizes a fingerprint part the same way as the original tool.
func Normalize(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

// MakeFingerprint builds the dedupe key: source::cwd::normalized text,
// truncated to 240 chars of text.
func MakeFingerprint(source, cwd, text string) string {
	sourcePart := Normalize(source)
	cwdPart := Normalize(cwd)
	textPart := Normalize(text)
	if len(textPart) > 240 {
		textPart = textPart[:240]
	}
	return sourcePart + "::" + cwdPart + "::" + textPart
}

// checkAndRemember returns true when an equivalent notification was already
// sent within the window. It always records the fingerprint (when not
// duplicated), pruning entries older than the window. Callers must hold the
// cross-process lock (CheckAndRemember) to make this race-free.
func (s *Store) checkAndRemember(fp string, windowMinutes int) (bool, error) {
	windowMs := int64(windowMinutes) * 60 * 1000
	if windowMs < minWindow {
		windowMs = minWindow
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	cutoff := now - windowMs
	kept := s.recent[:0]
	for _, e := range s.recent {
		if e.Timestamp >= cutoff {
			kept = append(kept, e)
		}
	}
	if len(kept) > maxEntries {
		kept = kept[len(kept)-maxEntries:]
	}
	s.recent = kept

	duplicate := false
	for _, e := range s.recent {
		if e.Fingerprint == fp {
			duplicate = true
			break
		}
	}
	if !duplicate {
		s.recent = append(s.recent, RecentNotification{Fingerprint: fp, Timestamp: now})
	}

	if err := s.persistLocked(); err != nil {
		// Do not fail the notification on a dedupe write error; the entry
		// stays in memory for the process lifetime.
		_ = err
	}
	return duplicate, nil
}

// Reset empties the store (used by tests).
func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recent = []RecentNotification{}
	_ = s.persistLocked()
}

// persistLocked writes the store to disk via a temp file + atomic rename so
// readers never observe a partially written state file.
func (s *Store) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(persistedState{RecentNotifications: s.recent}, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path)
}
