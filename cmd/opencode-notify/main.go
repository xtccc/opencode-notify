// Command opencode-notify is the CLI for the OpenCode notification
// bridge. All commands emit JSON on stdout except --help.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"opencode-notify/internal/config"
	"opencode-notify/internal/hook"
	"opencode-notify/internal/notify"
	"opencode-notify/internal/sound"
)

const (
	programName = "opencode-notify"
	version     = "0.1.0"
)

type app struct {
	flags map[string]string
	args  []string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		emitJSON(map[string]any{"ok": false, "error": err.Error()})
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("缺少子命令 (install | uninstall | status | notify | test | test-sound | config | version)")
	}
	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "install":
		return cmdInstall(rest)
	case "uninstall":
		return cmdUninstall(rest)
	case "status":
		return cmdStatus(rest)
	case "notify":
		return cmdNotify(rest)
	case "test":
		return cmdTest(rest)
	case "test-sound":
		return cmdTestSound(rest)
	case "config":
		return cmdConfig(rest)
	case "version":
		return emitJSON(map[string]any{"ok": true, "name": programName, "version": version})
	case "help", "-h", "--help":
		return printHelp()
	default:
		return fmt.Errorf("未知子命令 %q", cmd)
	}
}

// parseFlags consumes --key value / --key=value pairs. Boolean-ish flags
// (--force) take no value; unknown value-less flags are treated as toggles.
func parseFlags(args []string, valueFlags map[string]bool) map[string]string {
	flags := map[string]string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		body := strings.TrimPrefix(arg, "--")
		key, val, hasEq := strings.Cut(body, "=")
		if valueFlags[key] && !hasEq {
			if i+1 < len(args) {
				val = args[i+1]
				i++
			}
		}
		flags[key] = val
	}
	return flags
}

func flagBool(flags map[string]string, key string) bool {
	v, ok := flags[key]
	if !ok {
		return false
	}
	switch strings.ToLower(v) {
	case "", "1", "true", "yes":
		return true
	default:
		return false
	}
}

func cmdInstall(rest []string) error {
	exe := ownExecutable()
	path, err := hook.Install(exe)
	if err != nil {
		return err
	}
	return emitJSON(map[string]any{"ok": true, "mode": "install", "plugin": path, "executable": exe})
}

func cmdUninstall(rest []string) error {
	removed, err := hook.Uninstall()
	if err != nil {
		return err
	}
	return emitJSON(map[string]any{"ok": true, "mode": "uninstall", "removed": removed})
}

func cmdStatus(rest []string) error {
	st := hook.Status()
	return emitJSON(map[string]any{
		"ok":        true,
		"mode":      "status",
		"installed": st.Installed,
		"plugin":    st.PluginPath,
		"config":    st.SettingsPath,
	})
}

func cmdNotify(rest []string) error {
	valueFlags := map[string]bool{"source": true, "task": true, "duration-ms": true}
	flags := parseFlags(rest, valueFlags)

	opts := notify.Options{
		Source:     firstNonEmpty(flags["source"], "opencode"),
		TaskInfo:   flags["task"],
		FromHook:   flagBool(flags, "from-hook"),
		Force:      flagBool(flags, "force"),
		SkipDedupe: flagBool(flags, "skip-dedupe"),
		NoGotify:   flagBool(flags, "no-gotify"),
		NoSound:    flagBool(flags, "no-sound"),
	}
	if v := flags["duration-ms"]; v != "" {
		ms, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("duration-ms 无效: %w", err)
		}
		opts.DurationMs = &ms
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out := notify.Run(ctx, opts)

	payload := map[string]any{
		"ok":       out.OK,
		"skipped":  out.Skipped,
		"reason":   out.Reason,
		"mode":     "notify",
		"source":   out.Source,
		"kind":     out.Kind,
		"cwd":      out.Cwd,
		"project":  out.Project,
		"taskInfo": out.TaskInfo,
		"results":  out.Results,
	}
	return emitJSON(payload)
}

func cmdTest(rest []string) error {
	flags := parseFlags(rest, map[string]bool{"task": true})
	taskInfo := "测试通知"
	if flagBool(flags, "error") {
		taskInfo = "测试错误通知"
	}
	if v := flags["task"]; v != "" {
		taskInfo = v
	}

	opts := notify.Options{
		Source:     "opencode",
		TaskInfo:   taskInfo,
		SkipDedupe: true,
		NoGotify:   flagBool(flags, "no-gotify"),
		NoSound:    flagBool(flags, "no-sound"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out := notify.Run(ctx, opts)

	payload := map[string]any{
		"ok":       out.OK,
		"mode":     "test",
		"source":   out.Source,
		"taskInfo": out.TaskInfo,
		"results":  out.Results,
	}
	return emitJSON(payload)
}

func cmdTestSound(rest []string) error {
	flags := parseFlags(rest, map[string]bool{"task": true})
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Sound.Enabled = true
	if flagBool(flags, "tts") {
		cfg.Sound.TTS = true
		cfg.Sound.AudioPath = ""
	}
	if flagBool(flags, "beep") {
		cfg.Sound.TTS = false
		cfg.Sound.AudioPath = ""
	}
	text := firstNonEmpty(flags["task"], "测试声音")

	res := sound.Play(context.Background(), cfg.Sound, text)
	return emitJSON(map[string]any{
		"ok":       res.OK,
		"mode":     "test-sound",
		"provider": res.Provider,
		"modeUsed": res.Mode,
		"error":    res.Error,
	})
}

func cmdConfig(rest []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	redacted := cfg
	redacted.Gotify.AppToken = maskToken(redacted.Gotify.AppToken)
	redacted.Sound.MimoAPIKey = maskToken(redacted.Sound.MimoAPIKey)
	return emitJSON(map[string]any{
		"ok":     true,
		"mode":   "config",
		"path":   config.SettingsPath(),
		"config": redacted,
	})
}

func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 6 {
		return "***"
	}
	return token[:3] + "***" + token[len(token)-3:]
}

func ownExecutable() string {
	if exe, err := os.Executable(); err == nil {
		return exe
	}
	return "opencode-notify"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func emitJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func printHelp() error {
	help := `opencode-notify — OpenCode 任务完成通知桥

子命令:
  install                 生成并安装 opencode 插件 (默认使用当前可执行文件)
  uninstall               移除由本工具安装的插件
  status                  查看插件安装状态
  notify [flags]          发送通知 (插件通过 --from-hook 调用)

notify flags:
  --source NAME            来源标签 (默认 opencode)
  --task TEXT              任务描述 (覆盖默认文本)
  --duration-ms N          任务耗时 ms
  --from-hook              从 stdin 读取插件 JSON payload
  --force                  跳过 duration 阈值与去重
  --skip-dedupe            关闭去重
  --no-gotify              关闭 Gotify 通道
  --no-sound               关闭声音通道

其他:
  test [--error] [--no-gotify] [--no-sound]   发送一条测试通知
  test-sound [--tts] [--beep]                 仅测试声音通道
  config                                      打印脱敏后的生效配置
  version                                     打印版本
  help                                        本帮助

所有命令输出 JSON (stdout)。配置文件: ` + config.SettingsPath() + `
`
	fmt.Print(help)
	return nil
}
