// Package sound implements the Xiaomi Mimo TTS channel: request a WAV via
// the HTTP API, decode it, and play it through the local audio player chain.
package sound

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"opencode-notify/internal/config"
)

// mimoEndpoint is the Xiaomi Mimo TTS chat/completions endpoint.
const mimoEndpoint = "https://api.xiaomimimo.com/v1/chat/completions"

// mimoEndpointOverride redirects requests for tests ("" = use mimoEndpoint).
var mimoEndpointOverride = ""

// mimoDefaultVoice is used when no voice is configured.
const mimoDefaultVoice = "Chloe"

// mimoRequest mirrors the API request body.
type mimoRequest struct {
	Model    string        `json:"model"`
	Messages []mimoMessage `json:"messages"`
	Audio    mimoAudio     `json:"audio"`
}

type mimoMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type mimoAudio struct {
	Format string `json:"format"`
	Voice  string `json:"voice"`
}

// mimoResponse captures just the fields we need from the API response.
type mimoResponse struct {
	Choices []struct {
		Message struct {
			Audio struct {
				Data string `json:"data"`
			} `json:"audio"`
		} `json:"message"`
	} `json:"choices"`
}

// mimoTTS synthesizes `text` via the Xiaomi Mimo API and plays the result.
// It never blocks past ctx and never panics; failures fall back to the
// system beep when cfg.FallbackBeep is enabled.
func mimoTTS(ctx context.Context, cfg config.SoundConfig, text string) Result {
	key := strings.TrimSpace(cfg.MimoAPIKey)
	if key == "" {
		return beep("tts", ctx, cfg, "missing mimoApiKey")
	}
	voice := strings.TrimSpace(cfg.TTSVoice)
	if voice == "" {
		voice = mimoDefaultVoice
	}

	say := firstNonEmpty(text, "通知")
	body, err := json.Marshal(mimoRequest{
		Model: "mimo-v2.5-tts",
		Messages: []mimoMessage{
			{Role: "user", Content: "用自然、清晰、友好、语速适中的语气播报这条通知。"},
			{Role: "assistant", Content: say},
		},
		Audio: mimoAudio{Format: "wav", Voice: voice},
	})
	if err != nil {
		return beep("tts", ctx, cfg, "mimo payload error")
	}

	endpoint := mimoEndpoint
	if mimoEndpointOverride != "" {
		endpoint = mimoEndpointOverride
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return beep("tts", ctx, cfg, "mimo request error")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return beep("tts", ctx, cfg, "mimo 请求失败: "+err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return beep("tts", ctx, cfg,
			"mimo 返回 HTTP "+strconv.Itoa(resp.StatusCode)+": "+truncateStr(string(raw), 120))
	}

	var ar mimoResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&ar); err != nil {
		return beep("tts", ctx, cfg, "mimo 响应解析失败")
	}
	if len(ar.Choices) == 0 || ar.Choices[0].Message.Audio.Data == "" {
		return beep("tts", ctx, cfg, "mimo 响应缺少音频数据")
	}

	raw, err := base64.StdEncoding.DecodeString(ar.Choices[0].Message.Audio.Data)
	if err != nil || len(raw) == 0 {
		return beep("tts", ctx, cfg, "mimo 音频解码失败")
	}

	tmp, err := os.CreateTemp("", "opencode-notify-tts-*.wav")
	if err != nil {
		return beep("tts", ctx, cfg, "创建临时音频失败")
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return beep("tts", ctx, cfg, "写入临时音频失败")
	}
	if err := tmp.Close(); err != nil {
		return beep("tts", ctx, cfg, "写入临时音频失败")
	}

	program, args, ok := Resolve(ModePlay)
	if !ok {
		return beep("tts", ctx, cfg, "no audio player found")
	}
	res := runArgs(ctx, program, expandFile(args, path), "tts")
	if !res.OK {
		return beep("tts", ctx, cfg, "播放失败: "+res.Error)
	}
	return res
}

func truncateStr(text string, max int) string {
	value := strings.TrimSpace(text)
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}
