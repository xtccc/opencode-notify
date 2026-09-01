// Package gotify implements the Gotify push client (POST /message).
package gotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"opencode-notify/internal/config"
)

// Kind is the notification kind used for priority selection.
type Kind string

const (
	KindComplete Kind = "complete"
	KindError    Kind = "error"
	KindQuestion Kind = "question"
)

// Result is the outcome of one gotify request.
type Result struct {
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
	Status int    `json:"status,omitempty"`
}

type messagePayload struct {
	Title    string         `json:"title"`
	Message  string         `json:"message"`
	Priority int            `json:"priority"`
	Extras   map[string]any `json:"extras,omitempty"`
}

var (
	urlRe   = regexp.MustCompile(`https?://\S+`)
	tokenRe = regexp.MustCompile(`(?i)(apptoken|token|secret)=[^\s&]+`)
)

// Send pushes a notification to Gotify.
func Send(ctx context.Context, cfg config.GotifyConfig, title, message string, kind Kind) Result {
	if strings.TrimSpace(cfg.URL) == "" {
		return Result{OK: false, Error: "未配置 GOTIFY URL"}
	}
	if strings.TrimSpace(cfg.AppToken) == "" {
		return Result{OK: false, Error: "未配置 GOTIFY AppToken"}
	}

	base, err := url.Parse(cfg.URL)
	if err != nil {
		return Result{OK: false, Error: "GOTIFY URL 无效"}
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/message"

	priority := cfg.Priority.Complete
	switch kind {
	case KindError:
		priority = cfg.Priority.Error
	case KindQuestion:
		priority = cfg.Priority.Question
	}
	body, err := json.Marshal(messagePayload{
		Title:    title,
		Message:  message,
		Priority: priority,
		Extras: map[string]any{
			"client::display": map[string]any{
				"contentType": "text/markdown",
			},
		},
	})
	if err != nil {
		return Result{OK: false, Error: "payload marshal failed"}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base.String(), bytes.NewReader(body))
	if err != nil {
		return Result{OK: false, Error: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gotify-Key", cfg.AppToken)

	timeout := time.Duration(cfg.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	resp, err := client.Do(req)
	if err != nil {
		return Result{OK: false, Error: "请求失败: " + err.Error()}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{
			OK:     false,
			Status: resp.StatusCode,
			Error:  fmt.Sprintf("Gotify 返回 HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200)),
		}
	}
	return Result{OK: true, Status: resp.StatusCode}
}

// Redact strips secrets/URLs from an error string for safe logging.
func Redact(text string) string {
	out := tokenRe.ReplaceAllString(text, "$1=<redacted>")
	return urlRe.ReplaceAllString(out, "<redacted URL>")
}

func truncate(text string, max int) string {
	value := strings.TrimSpace(text)
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}
