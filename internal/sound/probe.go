// Package sound discovers sound-playing CLIs on disk.
package sound

import (
	"os/exec"
	"sync"
)

// Mode selects which capability we need.
type Mode string

const (
	ModeTTS  Mode = "tts"
	ModePlay Mode = "play" // play an audio file
	ModeBeep Mode = "beep"
)

// Provider describes one candidate in a fallback chain.
type Provider struct {
	Name string   // binary name, e.g. "paplay"
	Args []string // template args, may contain {FILE} / {TEXT}
}

// probeChains is the single source of truth for the fallback chains.
var probeChains = map[Mode][]Provider{
	ModeTTS: {
		{Name: "espeak-ng", Args: []string{"{TEXT}"}},
		{Name: "espeak", Args: []string{"{TEXT}"}},
		{Name: "spd-say", Args: []string{"{TEXT}"}},
	},
	ModePlay: {
		{Name: "paplay", Args: []string{"{FILE}"}},
		{Name: "pw-play", Args: []string{"{FILE}"}},
		{Name: "aplay", Args: []string{"{FILE}"}},
		{Name: "ffplay", Args: []string{"-nodisp", "-autoexit", "{FILE}"}},
		{Name: "mpv", Args: []string{"--no-terminal", "--really-quiet", "{FILE}"}},
	},
	ModeBeep: {
		{Name: "canberra-gtk-play", Args: []string{"-i", "bell"}},
		{Name: "paplay", Args: []string{bellFilePlaceholder}},
		{Name: "aplay", Args: []string{bellFilePlaceholder}},
	},
}

const bellFilePlaceholder = "BELL_FILE"

// freeDesktopBellCandidates are well-known bell sound paths.
var (
	freeDesktopBellCandidates = []string{
		"/usr/share/sounds/freedesktop/stereo/bell.oga",
		"/usr/share/sounds/freedesktop/stereo/message.oga",
	}
	probeMu    sync.Mutex
	probeCache = map[Mode]string{} // mode -> resolved provider name ("" = none)

	// lookPath is a seam for tests; production uses exec.LookPath.
	lookPath = func(name string) (string, error) { return exec.LookPath(name) }
)

// Resolve returns the first available provider in the chain for mode,
// caching the result. Returns ok=false when nothing is available.
func Resolve(mode Mode) (program string, args []string, ok bool) {
	probeMu.Lock()
	defer probeMu.Unlock()

	if cached, exists := probeCache[mode]; exists {
		if cached == "" {
			return "", nil, false
		}
		for _, p := range probeChains[mode] {
			if p.Name == cached {
				return p.Name, resolveArgs(p, mode), true
			}
		}
		return "", nil, false
	}

	for _, p := range probeChains[mode] {
		if path, err := lookPath(p.Name); err == nil && path != "" {
			probeCache[mode] = p.Name
			return p.Name, resolveArgs(p, mode), true
		}
	}
	probeCache[mode] = ""
	return "", nil, false
}

// resolveArgs turns a provider's template args into final args. The bell
// file placeholder is replaced with the first existing freedesktop bell
// file; when none exists the arg is dropped so callers can fall back to
// the terminal bell.
func resolveArgs(p Provider, mode Mode) []string {
	out := make([]string, 0, len(p.Args))
	for _, a := range p.Args {
		if mode == ModeBeep && a == bellFilePlaceholder {
			if path := firstBellFile(); path != "" {
				out = append(out, path)
			}
			continue
		}
		out = append(out, a)
	}
	return out
}

func firstBellFile() string {
	probeMu.Lock()
	defer probeMu.Unlock()
	for _, p := range freeDesktopBellCandidates {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// ResetCache clears the probe cache (used by tests).
func ResetCache() {
	probeMu.Lock()
	defer probeMu.Unlock()
	probeCache = map[Mode]string{}
}
