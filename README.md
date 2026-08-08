# opencode-notify

OpenCode 任务完成通知桥（Go 实现）。

当 OpenCode 会话结束时（成功 / 失败），通过 [Gotify](https://gotify.net) 推送通知到手机/桌面，并在本机播报声音。原 `ai-cli-complete-notify`（Node.js）的 Go 重写版，零第三方依赖，单一静态二进制。

## 特性

- **监听 OpenCode 完成事件**：`session.idle` / `session.error` / `session.status(idle)` / `question.asked`（助手提问等待回答），通过生成 opencode 插件 JS（Bun 生态）实现
- **Gotify 推送**：成功/失败不同优先级（默认 5 / 10），标题带项目名
- **声音播报**：自动探测系统 CLI（espeak-ng / paplay / aplay / canberra-gtk-play 等），支持 TTS、播放音频文件、beep 回退
- **去重**：内容指纹 + 时间窗（默认 5 分钟），避免重复推送
- **耗时阈值**：短任务不打扰（可配置）
- **安全**：AppToken 只在配置文件/环境变量中，日志输出全程脱敏
- **可脚本化**：所有命令 stdout 输出 JSON

## 安装

### 1. 构建二进制

```bash
go build -o opencode-notify ./cmd/opencode-notify   # 或 make build
```

产物为单一静态二进制（`CGO_ENABLED=0`），可放到任意路径：

```bash
make install          # 安装到 ~/.local/bin/opencode-notify
```

### 2. 配置

配置文件默认位置：`~/.config/opencode-notify/settings.json`

```bash
opencode-notify config   # 打印当前生效配置（token 脱敏）
```

参考 `config.example.json`，完整字段见下表：

| 字段 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `version` | int | 1 | 配置 schema 版本 |
| `gotify.url` | string | "" | Gotify 服务器地址，如 `https://gotify.example.com` |
| `gotify.appToken` | string | "" | Gotify 应用令牌（只推送） |
| `gotify.timeoutMs` | int | 10000 | HTTP 超时 |
| `gotify.priority.complete` | int | 5 | 成功通知优先级 |
| `gotify.priority.error` | int | 10 | 失败通知优先级 |
| `gotify.priority.question` | int | 6 | 助手提问等待回答时的通知优先级 |
| `opencode.enabled` | bool | true | 总开关 |
| `opencode.minDurationMinutes` | int | 0 | 耗时低于该分钟数的成功任务不通知（0=关闭） |
| `sound.enabled` | bool | true | 声音通道开关 |
| `sound.tts` | bool | true | true=语音播报任务文本；false=beep 或播放自定义文件 |
| `sound.audioPath` | string | "" | 非 TTS 时优先播放该音频文件（wav/mp3/oga） |
| `sound.staticText` | string | "" | 有值时语音播报该固定文本，否则播报 task_info |
| `sound.fallbackBeep` | bool | true | TTS/播放失败时回退系统提示音 |
| `sound.overrideCommand` | string | "" | 完全覆盖自动探测，支持 `$FILE`/`$TEXT` 占位符 |
| `dedupe.enabled` | bool | true | 内容去重开关 |
| `dedupe.windowMinutes` | int | 5 | 去重时间窗 |
| `ui.language` | string | "zh-CN" | UI 语言（预留） |

环境变量覆盖（优先级高于配置文件）：

```bash
export OPENCODE_NOTIFY_GOTIFY_URL="https://gotify.example.com"
export OPENCODE_NOTIFY_GOTIFY_TOKEN="xxxxx"          # 或写入 settings.json
export OPENCODE_NOTIFY_CONFIG_DIR="/path/to/config"  # 自定义配置目录
export OPENCODE_NOTIFY_STATE_DIR="/path/to/state"    # 自定义去重状态目录
```

### 3. 安装 opencode 插件

```bash
opencode-notify install
```

将生成插件并写入 `~/.config/opencode/plugins/opencode-notify.js`，插件内已烧入 `opencode-notify` 二进制绝对路径。重启 opencode 后生效。

如需覆盖二进制路径（例如升级后二进制换位置，不想重装插件）：

```bash
OPENCODE_NOTIFY_BIN=/new/path/opencode-notify opencode
```

### 4. 验证

```bash
opencode-notify test              # 发一条测试通知（gotify + 声音）
opencode-notify test --error      # 发一条"失败"测试通知（高优先级）
opencode-notify test-sound        # 只测声音通道
opencode-notify status            # 插件安装状态
```

## 用法

### CLI

```bash
opencode-notify install                   # 安装插件
opencode-notify uninstall                 # 卸载插件（仅删除本工具安装的文件）
opencode-notify status                    # 插件安装状态（JSON）
opencode-notify notify --from-hook        # 通知主命令（插件内部调用，通常不需要手动执行）
opencode-notify test [--error] [--no-gotify] [--no-sound]
opencode-notify test-sound [--tts] [--beep]
opencode-notify config                    # 打印脱敏配置
opencode-notify version
opencode-notify help
```

所有命令输出 JSON，便于脚本解析。

### 工作原理

```
opencode (Bun 运行时)
  └── ~/.config/opencode/plugins/opencode-notify.js  ← Go 生成/安装
        │  监听 session.idle / session.error / session.status(idle) / question.asked
        │  1.5s 窗口内事件去重
        │  Bun.spawn( <二进制> notify --source opencode --from-hook --force )
        │  stdin ─────▶ JSON payload {hook_event_name, cwd, task_info, ...}
        ▼
  opencode-notify (Go)
        ├── 读取 stdin JSON（1.5s 超时，非阻塞）
        ├── 解析事件 → kind(complete|error|question) / task_info
        ├── 加载配置 → 总开关 / 耗时阈值 / 去重检查
        ├── POST Gotify /message   （通道 1）
        └── 调用声音 CLI 播报       （通道 2，与 gotify 并行）
        └── stdout 输出 JSON 结果
```

## 声音自动探测回退链

| 场景 | 回退链 |
|---|---|
| TTS 语音 | `espeak-ng` → `espeak` → `spd-say` |
| 播放音频文件 | `paplay` → `pw-play` → `aplay` → `ffplay` → `mpv` |
| 纯 beep | `canberra-gtk-play -i bell` → freedesktop `bell.oga` → 终端 `\a` |

无音频设备 / 容器环境返回 `{ok:false, error}`，不阻塞 Gotify 通道。

## 开发

```bash
make build     # 构建二进制
make test      # go test ./...
make vet       # go vet ./...
```

目录结构：

```
cmd/opencode-notify/   CLI 入口（参数解析 + 命令分发）
internal/config/       配置加载/校验/保存 + 路径解析（XDG）
internal/hook/         install / uninstall / status（插件文件管理）
internal/plugin/       内嵌插件模板渲染（go:embed）
internal/hookcontext/  stdin payload 解析 → 通知上下文
internal/notify/       通知编排（配置→去重→通道并行分发）
internal/gotify/       Gotify 客户端（POST /message）
internal/sound/        声音通道（TTS / 播放文件 / beep + 探测）
internal/format/       时长/标题/时间格式化
internal/state/        去重状态文件
```

## 常见问题

**Q: `notify` 返回 `skipped:true, reason:"opencode 通知已禁用"`**
A: 检查 `opencode.enabled` 是否为 `false`（注意：配置文件里缺失的 section 会回退默认值，不存在"零值导致禁用"的问题；`config` 命令可确认）。

**Q: Gotify 推送 401**
A: 确认 `appToken` 正确。app token 只能推送，不能用它读取消息 API（读取需 client token 或用户凭证）。

**Q: 插件已安装但不触发**
A: 确认安装插件后**重启了 opencode**；确认 `status` 显示 `installed:true`；可用 `test` 命令验证通道本身正常。

**Q: 声音没响**
A: 运行 `opencode-notify test-sound` 看探测结果；无桌面音频环境（如容器）会失败但**不影响 Gotify 推送**。

## License

MIT
