// Package sound plays notification sounds by invoking local CLI tools
// (discovered via probe). It never blocks longer than soundTimeout.
package sound

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"opencode-notify/internal/config"
)

// Result is the outcome of one sound attempt.
type Result struct {
	OK       bool   `json:"ok"`
	Provider string `json:"provider,omitempty"`
	Mode     string `json:"mode,omitempty"`
	Error    string `json:"error,omitempty"`
}

const soundTimeout = 10 * time.Second

// Play speaks/plays/beeps the given text according to the config. It always
// returns without panicking; errors are captured in the result.
func Play(ctx context.Context, cfg config.SoundConfig, text string) Result {
	if !cfg.Enabled {
		return Result{OK: false, Mode: "tts", Error: "sound channel disabled"}
	}
	execCtx, cancel := context.WithTimeout(ctx, soundTimeout)
	defer cancel()

	if override := strings.TrimSpace(cfg.OverrideCommand); override != "" {
		return runOverride(execCtx, override, cfg, text)
	}

	if cfg.TTS {
		program, args, ok := Resolve(ModeTTS)
		if !ok {
			return beep("tts", execCtx, cfg, "no TTS tool found")
		}
		sayText := firstNonEmpty(cfg.StaticText, text, "通知")
		return runArgs(execCtx, program, expandTTS(args, sayText), "tts")
	}

	if file := strings.TrimSpace(cfg.AudioPath); file != "" {
		if !fileExists(file) {
			return beep("play", execCtx, cfg, "audio file not found: "+file)
		}
		program, args, ok := Resolve(ModePlay)
		if !ok {
			return beep("play", execCtx, cfg, "no audio player found")
		}
		return runArgs(execCtx, program, expandFile(args, file), "play")
	}

	return beep("beep", execCtx, cfg, "no audio path configured")
}

// runOverride executes a user-specified command template supporting
// $FILE / ${FILE} / $TEXT / ${TEXT} placeholders. The string is split with
// strings.Fields (no shell evaluation).
func runOverride(ctx context.Context, override string, cfg config.SoundConfig, text string) Result {
	content := firstNonEmpty(cfg.StaticText, text, "通知")
	file := cfg.AudioPath

	replaced := strings.ReplaceAll(override, "$FILE", file)
	replaced = strings.ReplaceAll(replaced, "${FILE}", file)
	replaced = strings.ReplaceAll(replaced, "$TEXT", content)
	replaced = strings.ReplaceAll(replaced, "${TEXT}", content)

	parts := strings.Fields(replaced)
	if len(parts) == 0 {
		return Result{OK: false, Mode: "override", Error: "empty override command"}
	}
	return runArgs(ctx, parts[0], parts[1:], "override")
}

// runArgs spawns the program with the final args. Text/file placeholders
// are passed as separate argv entries (no shell interpolation).
func runArgs(ctx context.Context, program string, args []string, mode string) Result {
	cmd := exec.CommandContext(ctx, program, args...)
	var sb strings.Builder
	cmd.Stdout = &sb
	cmd.Stderr = &sb
	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return Result{OK: false, Provider: program, Mode: mode, Error: "sound timeout"}
		}
		return Result{OK: false, Provider: program, Mode: mode, Error: "命令失败: " + err.Error()}
	}
	return Result{OK: true, Provider: program, Mode: mode}
}

// beep tries a system beep when the primary mode failed, honoring the
// fallbackBeep flag.
func beep(failedMode string, ctx context.Context, cfg config.SoundConfig, reason string) Result {
	if !cfg.FallbackBeep {
		return Result{OK: false, Mode: failedMode, Error: reason}
	}
	program, args, ok := Resolve(ModeBeep)
	if !ok {
		return Result{OK: false, Mode: "beep", Error: reason + "; no beep tool found"}
	}
	if len(args) == 0 {
		// Terminal bell as a last resort.
		_, _ = os.Stdout.Write([]byte("\a"))
		return Result{OK: true, Provider: "bell", Mode: "beep"}
	}
	res := runArgs(ctx, program, args, "beep")
	if !res.OK {
		res.Error = reason + "; beep failed: " + res.Error
		return res
	}
	return res
}

// expandTTS replaces the {TEXT} placeholder in TTS args with the text.
func expandTTS(args []string, text string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		out = append(out, strings.ReplaceAll(a, "{TEXT}", text))
	}
	return out
}

// expandFile replaces the {FILE} placeholder in player args with the path.
func expandFile(args []string, file string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		out = append(out, strings.ReplaceAll(a, "{FILE}", file))
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
