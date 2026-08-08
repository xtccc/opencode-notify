# OpenCode Notify — 设计文档（Go）

## 1. 背景与目标

原 `ai-cli-complete-notify`（Node.js）是一个覆盖 Claude / Codex / Gemini / OpenCode 的多通道任务完成通知系统。本次使用 Go 重写，聚焦最小可用范围：

- **唯一集成对象**：OpenCode（通过生成 opencode 插件 JS 监听 `session.idle` / `session.error` / `session.status` / `question.asked` 事件）
- **通知通道**：Gotify（自托管推送）+ Sound（调用系统声音 CLI 播报）
- **平台**：Linux 优先（跨平台保留扩展点，但不实现 Windows/macOS 专用逻辑）
- **配置**：全新 JSON 格式（不复用旧 settings.json / .env），命名空间独立

### 不包含（明确排除）
- Tauri 桌面 GUI、React 前端
- Claude / Codex / Gemini 的 hook 与 watch 日志轮询引擎
- webhook / telegram / email / desktop 等其它通道
- AI 摘要、focus 窗口聚焦

## 2. 整体架构

```
opencode (Bun 运行时)
└── ~/.config/opencode/plugins/opencode-notify.js   ← Go 生成/安装的插件
        │  监听 session.idle / session.error / session.status(idle) / question.asked
        │  1.5s 内去重（event::session_id::error）
        │  Bun.spawn( 绝对路径/opencode-notify notify --source opencode --from-hook --force )
        │  stdin ──────────────────▶ JSON payload（见 §6）
        ▼
  opencode-notify (Go 静态二进制)
        ├── 读取 stdin JSON（1.5s 超时，非阻塞）
        ├── 解析 hook 上下文 → kind(complete|error|question) / task_info
        ├── 加载配置 ~/.config/opencode-notify/settings.json
        ├── 去重（可选，内容指纹 + 时间窗）
        ├── POST Gotify /message            ← 通道 1：gotify（§8）
        └── 调用系统声音 CLI 播报            ← 通道 2：sound（§10）
        └── stdout 输出 JSON 结果
```

## 3. 目录结构

```
/home/xtcc/opencode-notify/
├── go.mod / go.sum              # 零第三方依赖（仅标准库），go >= 1.21
├── Makefile                     # build / test / vet / install
├── README.md
├── cmd/opencode-notify/main.go  # 入口：参数解析 + 命令分发
└── internal/
    ├── config/
    │   ├── paths.go             # XDG 路径解析（数据/配置目录、env 覆盖）
    │   └── config.go            # settings.json(v1) 加载/校验/保存 + 环境覆盖
    ├── hook/
    │   └── hook.go              # install / uninstall / status（插件文件管理）
    ├── plugin/
    │   ├── plugin.go            # go:embed 模板 → 渲染最终插件 JS
    │   └── template.js          # 内嵌的 opencode 插件模板（embed）
    ├── hookcontext/
    │   └── hookcontext.go       # 解析 stdin payload → 通知上下文
    ├── notify/
    │   └── notify.go            # notify 命令编排（配置→去重→通道并行分发）
    ├── gotify/
    │   └── gotify.go            # Gotify 客户端（POST /message）
    ├── sound/
    │   ├── sound.go             # 通道编排：TTS / 播放文件 / beep
    │   └── probe.go             # 播放 CLI 自动探测（exec.LookPath + 缓存）
    ├── format/
    │   └── format.go            # 时长/标题/来源标签格式化
    └── state/
        └── state.go             # 去重状态文件（state.json）
```

## 4. 配置设计（新 JSON 格式，v1）

**路径解析优先级**：
1. 环境变量 `OPENCODE_NOTIFY_CONFIG_DIR`（覆盖配置目录）
2. `$XDG_CONFIG_HOME/opencode-notify/settings.json`（默认 `~/.config/opencode-notify/`）
3. 状态目录 `$XDG_STATE_HOME/opencode-notify/`（默认 `~/.local/state/opencode-notify/`）

```jsonc
{
  "version": 1,
  "gotify": {
    "url": "https://gotify.example.com",
    "appToken": "",
    "timeoutMs": 10000,
    "priority": { "complete": 5, "error": 10, "question": 6 }
  },
  "opencode": {
    "enabled": true,
    "minDurationMinutes": 0
  },
  "sound": {
    "enabled": true,
    "tts": true,                // true=小米 TTS 播报；false=beep 或播放自定义文件
    "mimoApiKey": "",           // 小米 Mimo API key（也可用环境变量）
    "ttsVoice": "Chloe",        // 小米音色
    "audioPath": "",            // 非 TTS 时优先播放自定义音频文件（wav/mp3/oga）
    "staticText": "",           // 有值时语音播报该固定文本，否则播报 task_info
    "fallbackBeep": true,       // TTS/播放失败时回退系统提示音
    "overrideCommand": ""       // 可选：完全覆盖自动逻辑，支持 $FILE/$TEXT 占位符
  },
  "dedupe": {
    "enabled": true,
    "windowMinutes": 5
  },
  "ui": { "language": "zh-CN" }
}
```

- 环境变量覆盖：`OPENCODE_NOTIFY_GOTIFY_URL`、`OPENCODE_NOTIFY_GOTIFY_TOKEN`、`OPENCODE_NOTIFY_MIMO_API_KEY`、`OPENCODE_NOTIFY_CONFIG_DIR`。
- `appToken` / `mimoApiKey` 只允许来自配置文件或环境变量，**绝不写入日志**（错误统一脱敏，`config` 命令输出掩码）。

## 5. CLI 命令

| 命令 | 说明 |
|---|---|
| `opencode-notify install` | 生成并写入 opencode 插件到 `~/.config/opencode/plugins/opencode-notify.js` |
| `opencode-notify uninstall` | 按标记删除该插件文件 |
| `opencode-notify status` | 插件安装状态 + 配置状态（JSON 输出） |
| `opencode-notify notify --source opencode [--from-hook] [--force] [--task "..."] [--duration-ms N]` | 核心通知命令；`--from-hook` 时从 stdin 读 payload |
| `opencode-notify test [--error] [--no-gotify] [--no-sound]` | 发一条测试通知：gotify + 声音（默认同时触发，flag 可单独关闭） |
| `opencode-notify test-sound [--tts] [--beep]` | 只测声音通道：探测到的 CLI、播放是否成功 |
| `opencode-notify config` | 打印当前生效配置（脱敏） |
| `opencode-notify version` | 版本号 |

所有命令 stdout 输出 **JSON**（`{ok, mode, result}`），便于被插件/脚本安全调用与解析；`--help` 输出人类可读用法。

## 6. OpenCode 插件与事件契约

沿用「生成 JS 插件」机制（opencode 插件 API 为 JS/Bun 生态），但 spawn 的是 Go 二进制。

**插件模板**（`plugin/template.js`，由 Go 渲染）：
- 顶部标记 `// opencode-notify:plugin`（供 uninstall 识别）
- 内嵌 `NOTIFY_CMD`：`["<opencode-notify绝对路径>", "notify", "--source", "opencode", "--from-hook", "--force"]`；支持 `OPENCODE_NOTIFY_BIN` 环境变量覆盖二进制路径
- 导出 `OpenCodeNotifyPlugin = async ({ client, project, directory, worktree }) => ({ event: ... })`
- `isCompletionEvent`：`session.idle` / `session.error` / `session.status` 且 `status.type==='idle'` / `question.asked` / `question.v2.asked`
- 事件去重：`eventName::session_id::error_message`，1.5s 窗口
- **payload 构建**：idle 时调用 `client.session.messages({path:{id}}) ` 尽力拉取最后一条 assistant 文本（失败降级为空，不影响通知）；error 直接取 `error_message`；question 事件取 `properties.questions[0].question`，`task_info = "OpenCode 需要你回答: <问题>"`
- `Bun.spawn` 写入 stdin JSON，`stdout/stderr: 'ignore'`，不阻塞 opencode

**stdin JSON 契约（payload）**：

```jsonc
{
  "hook_source": "opencode-plugin",
  "hook_event_name": "session.idle",     // session.idle | session.error | session.status | question.asked | question.v2.asked
  "cwd": "/path/to/project",
  "task_info": "OpenCode 完成",
  "session_id": "sess_xxx",
  "project_name": "my-project",
  "error_message": "",
  "assistant_message": "最后一条助手回复...",
  "question_text": "需要你回答的问题文本（question 事件时）",
  "output_content": "最后一条助手回复...（question 事件时为问题文本）"
}
```

**Go 侧 hookcontext 解析规则**：
- `hook_event_name == "session.error"` → `kind=error`，`task_info = "OpenCode 失败: <error_message 截断 88 字符>"`
- `hook_event_name == "question.asked" / "question.v2.asked"` → `kind=question`，`task_info = "OpenCode 需要你回答[: <问题 截断 88 字符>]"`
- `session.idle` / `session.status` → `kind=complete`，`task_info = "OpenCode 完成"`（`--task` 显式传入则优先）
- 其他/空 event → 跳过（`skipped`）

## 7. notify 流程

1. 解析 flags；`--from-hook` 时读 stdin JSON（1.5s 超时，非阻塞，不依赖 TTY 状态）
2. 加载配置；`opencode.enabled == false` → `{skipped, reason}`
3. 阈值过滤：`minDurationMinutes > 0` 且 `!force` 且耗时不足 → skipped
4. 计算 `cwd`（payload.cwd 优先）、`projectName`（cwd 目录名）、`durationText`
5. 可选去重：内容指纹（`source::cwd::task_info` 规范化）写入 `state.json`，时间窗内重复 → skipped
6. 构建 Gotify 消息：title `[OpenCode] {project}` + error/question 后缀；body 含 `Completed at/Failed at/Awaiting user input at + 时间 + Duration + Source`
6b. 若 `sound.enabled` → `internal/sound`（与 gotify 并行 `errgroup`），结果 `{channel:'sound', ...}` 并入 results
7. stdout 输出 `{skipped:false, results:[{channel:'gotify',...},{channel:'sound',...}]}`

## 8. Gotify 通道（internal/gotify）

- 端点：`POST {url}/message`，body `{"title","message","priority"}`
- 鉴权：header `X-Gotify-Key: <appToken>`（若 url 自带 `token=` query 则兼容并存）
- priority：error=10 / complete=5 / question=6（可配置）
- 错误处理：非 2xx 或 JSON 解析失败 → `{ok:false, error}`；错误信息脱敏（URL、token 打码）
- 实现只用 `net/http`，无第三方依赖

## 9. 去重（internal/state，可选）

- 状态文件：`~/.local/state/opencode-notify/state.json`（结构 `{recentNotifications:[{fingerprint,timestamp}]}`，截断 200 条）
- 指纹：`source::cwd::normalized(task_info|output_content)[:240]`
- 窗口默认 5 分钟，`dedupe.enabled=false` 关闭

## 10. Sound 通道（internal/sound，Linux 原生）

**TTS（`tts=true`）走小米 Mimo 云语音**（`mimo.go`）：
- `POST https://api.xiaomimimo.com/v1/chat/completions`，`model=mimo-v2.5-tts`，`audio.format=wav`，`audio.voice`（默认 Chloe）
- key 从 `sound.mimoApiKey`（settings.json）或环境变量 `OPENCODE_NOTIFY_MIMO_API_KEY` 读取
- 解析 `choices[0].message.audio.data`（base64 WAV）→ 解码 → 临时文件 → 本地播放链 → 清理
- 失败（无 key / 网络 / 非 2xx / 解码失败 / 播放失败）→ `fallbackBeep` 回退 beep

**播放链**（`probe.go`，首次调用时 `exec.LookPath` 探测并缓存）：

| 场景 | 回退链 |
|---|---|
| 播放 WAV/音频文件 | `paplay` → `pw-play` → `aplay` → `ffplay -nodisp -autoexit` → `mpv --no-terminal` |
| 纯 beep | `canberra-gtk-play -i bell` → 自由桌面 `bell.oga`(paplay) → 终端 `\a` |

- **tts=false 且 `audioPath` 存在**：播放该文件；文件不存在或 CLI 缺失 → 回退 beep / `ok:false`
- **overrideCommand 非空**：跳过自动逻辑，`$FILE` / `$TEXT` 占位符替换后直接执行（参数透传，不做 shell 拼接）
- context 10s 超时；无音频设备 / 容器环境返回 `{ok:false, error}`，不阻塞 gotify 通道
- 结果并入 notify 输出的 `results`：`{channel:'sound', ok, provider, error?}`

## 11. 测试计划

- `config`：默认值、环境覆盖、非法 JSON 降级、token 脱敏（单测）
- `hookcontext`：三类事件映射、error 截断、未知事件 skip（表驱动单测）
- `gotify`：`httptest` server 验证请求头/路径/body、非 2xx、超时、脱敏
- `sound`：`httptest` 假 Mimo 服务端 + fake exec，验证请求体/解码/临时文件/播放/清理、失败兜底
- `plugin`：模板渲染包含 `NOTIFY_CMD` 与 marker；`hook` install/uninstall/status 集成测试（TMPDIR 隔离的 fake 配置目录）
- `notify`：端到端（fake gotify server + 管道喂 stdin payload）验证 JSON 输出
- 命令：`go test ./...`；`go vet ./...`

## 12. 构建与发布

- `CGO_ENABLED=0 go build -o opencode-notify ./cmd/opencode-notify` → 单一静态二进制
- `make install` 安装到 `~/.local/bin`
- Linux 桌面依赖：无（Gotify 走 HTTP；声音走探测到的 CLI）

## 13. 关键决策记录

| 决策 | 选择 | 理由 |
|---|---|---|
| 语言/依赖 | Go 标准库零依赖 | 静态二进制、易分发 |
| 集成方式 | 沿用 JS 插件 spawn Go 二进制 | opencode 插件 API 是 JS 生态 |
| 配置 | 新 JSON v1，独立命名空间 | 不污染/不复用旧格式 |
| 平台 | Linux 优先 | Gotify 纯 HTTP + 声音 CLI 探测 |
| 输出 | 全程 stdout JSON | 可被插件/脚本安全解析 |
| 去重 | 文件持久化可选 | 防止误报重复推送 |
| 声音 | TTS + beep/文件，自动探测回退链 | 无平台绑定，最少配置 |

## 14. 实施步骤

1. 初始化 `go.mod`、目录骨架、Makefile
2. `internal/config`（路径 + settings.json v1 + 环境覆盖）
3. `internal/format` + `internal/hookcontext`（payload 解析与任务分类）
4. `internal/gotify`（客户端 + 单测）
4b. `internal/sound`（探测 + TTS/文件/beep + 单测）
5. `internal/notify`（编排 + 去重 + JSON 输出）
6. `internal/plugin` + `internal/hook`（模板渲染、install/uninstall/status）
7. `cmd/opencode-notify`（CLI 分发 + test/test-sound）
8. 端到端手动验证（真实 opencode + Gotify）
9. 补齐单测、README、示例配置
