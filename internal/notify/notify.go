// Package notify orchestrates the `notify` command: config -> dedupe ->
// parallel channel dispatch -> JSON outcome.
package notify

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"opencode-notify/internal/config"
	"opencode-notify/internal/format"
	"opencode-notify/internal/gotify"
	"opencode-notify/internal/hookcontext"
	"opencode-notify/internal/sound"
	"opencode-notify/internal/state"
)

// Options control a single notify run.
type Options struct {
	Source     string
	FromHook   bool
	Force      bool
	TaskInfo   string
	DurationMs *int64
	SkipDedupe bool
	NoGotify   bool
	NoSound    bool

	// Stdin is used for --from-hook payloads; defaults to os.Stdin.
	Stdin io.Reader
}

// ChannelResult is the outcome of one delivery channel.
type ChannelResult struct {
	Channel  string `json:"channel"`
	OK       bool   `json:"ok"`
	Provider string `json:"provider,omitempty"`
	Mode     string `json:"mode,omitempty"`
	Status   int    `json:"status,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Outcome is the JSON result emitted on stdout.
type Outcome struct {
	OK       bool            `json:"ok"`
	Skipped  bool            `json:"skipped,omitempty"`
	Reason   string          `json:"reason,omitempty"`
	Mode     string          `json:"mode"`
	Source   string          `json:"source,omitempty"`
	Kind     string          `json:"kind,omitempty"`
	Cwd      string          `json:"cwd,omitempty"`
	Project  string          `json:"project,omitempty"`
	TaskInfo string          `json:"taskInfo,omitempty"`
	Results  []ChannelResult `json:"results,omitempty"`
}

const stdinTimeout = 1500 * time.Millisecond

// Run executes a notify flow and returns the outcome (never panics).
func Run(ctx context.Context, opts Options) Outcome {
	cfg, err := config.Load()
	if err != nil {
		return Outcome{OK: false, Mode: "notify", Reason: "配置加载失败: " + gotify.Redact(err.Error())}
	}
	if !cfg.OpenCode.Enabled {
		return Outcome{OK: true, Skipped: true, Mode: "notify", Reason: "opencode 通知已禁用 (opencode.enabled=false)"}
	}

	decision, skipReason := buildDecision(opts)
	if decision == nil {
		return Outcome{OK: true, Skipped: true, Mode: "notify", Reason: skipReason}
	}

	cwd := decision.Cwd
	if cwd == "" {
		cwd = detectCwd()
	}
	project := decision.ProjectName
	if project == "" && cwd != "" {
		project = filepath.Base(cwd)
	}

	// Duration threshold filter (skips short completed tasks).
	if !opts.Force && decision.Kind == hookcontext.KindComplete && opts.DurationMs != nil && *opts.DurationMs >= 0 {
		minMs := int64(cfg.OpenCode.MinDurationMinutes) * 60 * 1000
		if minMs > 0 && *opts.DurationMs < minMs {
			return Outcome{
				OK: true, Skipped: true, Mode: "notify", Source: opts.Source,
				Kind: string(decision.Kind), Cwd: cwd, Project: project, TaskInfo: decision.TaskInfo,
				Reason: formatDurationReason(*opts.DurationMs, cfg.OpenCode.MinDurationMinutes),
			}
		}
	}

	// Dedupe (optional, content fingerprint within a time window).
	if cfg.Dedupe.Enabled && !opts.SkipDedupe && !opts.Force {
		sig := decision.Signature
		if strings.TrimSpace(sig) == "" {
			sig = decision.TaskInfo
		}
		fp := state.MakeFingerprint(opts.Source, cwd, sig)
		dup, _ := state.CheckAndRemember(fp, cfg.Dedupe.WindowMinutes)
		if dup {
			return Outcome{
				OK: true, Skipped: true, Mode: "notify", Source: opts.Source,
				Kind: string(decision.Kind), Cwd: cwd, Project: project, TaskInfo: decision.TaskInfo,
				Reason: "重复通知 (去重窗口内已发送过相同内容)",
			}
		}
	}

	label := format.SourceLabel(opts.Source)
	title := format.BuildTitle(project, decision.TaskInfo, label)
	message := buildMessage(decision.Kind, opts.DurationMs, label, cwd, decision.TaskInfo, decision.OutputText)

	results := dispatch(ctx, cfg, opts, decision, title, message)
	ok := anyOK(results)
	if ok && len(results) == 0 {
		ok = false
	}

	return Outcome{
		OK:       ok,
		Mode:     "notify",
		Source:   opts.Source,
		Kind:     string(decision.Kind),
		Cwd:      cwd,
		Project:  project,
		TaskInfo: decision.TaskInfo,
		Results:  results,
	}
}

// buildDecision resolves the notification decision from hook stdin or from
// CLI-provided flags. A nil decision means "skip".
func buildDecision(opts Options) (*hookcontext.Decision, string) {
	if opts.FromHook {
		reader := opts.Stdin
		if reader == nil {
			reader = os.Stdin
		}
		payload, err := hookcontext.ReadStdinJSON(reader, stdinTimeout)
		if err != nil {
			return nil, "读取 hook 输入失败: " + err.Error()
		}
		d, err := hookcontext.Build(payload, opts.TaskInfo)
		if err != nil {
			return nil, "hook 载荷解析失败: " + err.Error()
		}
		if d.Skip {
			return nil, d.SkipReason
		}
		return d, ""
	}

	taskInfo := strings.TrimSpace(opts.TaskInfo)
	if taskInfo == "" {
		taskInfo = "OpenCode 完成"
	}
	return &hookcontext.Decision{Kind: hookcontext.KindComplete, TaskInfo: taskInfo}, ""
}

func detectCwd() string {
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return ""
}

func buildMessage(kind hookcontext.Kind, durationMs *int64, sourceLabel, cwd, taskInfo, outputText string) string {
	var sb strings.Builder
	switch kind {
	case hookcontext.KindError:
		sb.WriteString("## 出错于: " + format.Timestamp(time.Now()))
	case hookcontext.KindQuestion:
		sb.WriteString("## 等待回答于: " + format.Timestamp(time.Now()))
	default:
		sb.WriteString("## 完成于: " + format.Timestamp(time.Now()))
	}
	if cwd != "" {
		sb.WriteString("\n\n## 目录: " + cwd)
	}
	if taskInfo != "" {
		task := strings.TrimSpace(taskInfo)
		sb.WriteString("\n\n## 任务: " + task)
	}
	if d := format.FormatDurationMs(durationMs); d != "" {
		sb.WriteString("\n\n耗时: " + d)
	}
	if summary := strings.TrimSpace(outputText); summary != "" {
		sb.WriteString("\n\n## 结果:\n\n" + summary)
	}
	sb.WriteString("\n\n## 来源: " + sourceLabel)
	return sb.String()
}

func dispatch(ctx context.Context, cfg config.Config, opts Options, decision *hookcontext.Decision, title, message string) []ChannelResult {
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results = make([]ChannelResult, 0, 2)
	)

	if !opts.NoGotify {
		wg.Add(1)
		go func() {
			defer wg.Done()
			kind := gotify.KindComplete
			switch decision.Kind {
			case hookcontext.KindError:
				kind = gotify.KindError
			case hookcontext.KindQuestion:
				kind = gotify.KindQuestion
			}
			res := gotify.Send(ctx, cfg.Gotify, title, message, kind)
			mu.Lock()
			results = append(results, ChannelResult{
				Channel: "gotify", OK: res.OK, Status: res.Status, Error: res.Error,
			})
			mu.Unlock()
		}()
	}

	if cfg.Sound.Enabled && !opts.NoSound {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := sound.Play(ctx, cfg.Sound, decision.TaskInfo)
			mu.Lock()
			results = append(results, ChannelResult{
				Channel: "sound", OK: res.OK, Provider: res.Provider, Mode: res.Mode, Error: res.Error,
			})
			mu.Unlock()
		}()
	}

	wg.Wait()
	return results
}

func anyOK(results []ChannelResult) bool {
	for _, r := range results {
		if r.OK {
			return true
		}
	}
	return false
}

func formatDurationReason(ms int64, minMinutes int) string {
	return "任务耗时不足最小阈值 (" + format.FormatDurationMs(&ms) + " < " + format.FormatDurationMs(int64Ptr(int64(minMinutes)*60*1000)) + ")"
}

func int64Ptr(v int64) *int64 { return &v }
