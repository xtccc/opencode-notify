// Package state provides a small JSON-file-backed dedupe store.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// CheckAndRemember returns true when an equivalent notification was already
// sent within the window. It always records the fingerprint (when not
// duplicated), pruning entries older than the window.
func (s *Store) CheckAndRemember(fingerprint string, windowMinutes int) (bool, error) {
	if strings.TrimSpace(fingerprint) == "" {
		return false, nil
	}
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
		if e.Fingerprint == fingerprint {
			duplicate = true
			break
		}
	}
	if !duplicate {
		s.recent = append(s.recent, RecentNotification{Fingerprint: fingerprint, Timestamp: now})
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

func (s *Store) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(persistedState{RecentNotifications: s.recent}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, append(data, '\n'), 0o644)
}
