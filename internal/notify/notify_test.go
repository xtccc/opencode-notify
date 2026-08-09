package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"opencode-notify/internal/config"
)

// setupTestEnv points config/state dirs at temp dirs and returns cleanup.
func setupTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("OPENCODE_NOTIFY_CONFIG_DIR", filepath.Join(t.TempDir(), "config"))
	t.Setenv("OPENCODE_NOTIFY_STATE_DIR", filepath.Join(t.TempDir(), "state"))
}

// writeConfig writes a minimal settings.json disabling sound and pointing
// gotify at the given URL.
func writeConfig(t *testing.T, url string, dedupeEnabled bool) {
	t.Helper()
	cfg := config.Default()
	cfg.Gotify.URL = url
	cfg.Gotify.AppToken = "test-app-token"
	cfg.Sound.Enabled = false
	cfg.Dedupe.Enabled = dedupeEnabled
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
}

type gotifyRecorder struct {
	mu       sync.Mutex
	requests []gotifyRequest
	server   *httptest.Server
}

type gotifyRequest struct {
	Title    string `json:"title"`
	Message  string `json:"message"`
	Priority int    `json:"priority"`
	AuthKey  string
}

func newGotifyRecorder(t *testing.T) *gotifyRecorder {
	r := &gotifyRecorder{}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(req.Body)
		var gr gotifyRequest
		_ = json.Unmarshal(body, &gr)
		gr.AuthKey = req.Header.Get("X-Gotify-Key")
		r.mu.Lock()
		r.requests = append(r.requests, gr)
		r.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	t.Cleanup(r.server.Close)
	return r
}

func TestNotifyFromHookComplete(t *testing.T) {
	setupTestEnv(t)
	rec := newGotifyRecorder(t)
	writeConfig(t, rec.server.URL, true)

	payload := `{"hook_source":"opencode-plugin","hook_event_name":"session.idle","cwd":"/work/app","task_info":"OpenCode 完成","session_id":"s1","output_content":"hello world"}`
	opts := Options{Source: "opencode", FromHook: true, SkipDedupe: false, Stdin: strings.NewReader(payload)}

	out := Run(context.Background(), opts)
	if out.Skipped {
		t.Fatalf("should not skip: %+v", out)
	}
	if !out.OK {
		t.Fatalf("notify failed: %+v", out)
	}
	if out.Kind != "complete" {
		t.Errorf("kind = %q", out.Kind)
	}
	if out.Project != "app" {
		t.Errorf("project = %q", out.Project)
	}
	if len(out.Results) != 1 || out.Results[0].Channel != "gotify" {
		t.Fatalf("results = %+v", out.Results)
	}
	if !out.Results[0].OK {
		t.Errorf("gotify result: %+v", out.Results[0])
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.requests) != 1 {
		t.Fatalf("gotify requests = %d", len(rec.requests))
	}
	req := rec.requests[0]
	if req.AuthKey != "test-app-token" {
		t.Errorf("auth key = %q", req.AuthKey)
	}
	if !strings.Contains(req.Title, "[OpenCode]") {
		t.Errorf("title = %q", req.Title)
	}
	if !strings.Contains(req.Title, "app") {
		t.Errorf("title = %q", req.Title)
	}
	if !strings.Contains(req.Message, "目录: /work/app") {
		t.Errorf("message missing 目录: %q", req.Message)
	}
	if !strings.Contains(req.Message, "任务: OpenCode 完成") {
		t.Errorf("message missing 任务: %q", req.Message)
	}
	if !strings.Contains(req.Message, "结果: hello world") {
		t.Errorf("message missing 结果: %q", req.Message)
	}
	if !strings.Contains(req.Message, "完成于:") {
		t.Errorf("message missing 完成于: %q", req.Message)
	}
	if req.Priority != 5 {
		t.Errorf("priority = %d", req.Priority)
	}
}

func TestNotifyHookQuestion(t *testing.T) {
	setupTestEnv(t)
	rec := newGotifyRecorder(t)
	writeConfig(t, rec.server.URL, true)

	payload := `{"hook_source":"opencode-plugin","hook_event_name":"question.asked","cwd":"/app","session_id":"s-q1","question_text":"继续部署吗?","output_content":"继续部署吗?"}`
	opts := Options{Source: "opencode", FromHook: true, Stdin: strings.NewReader(payload)}

	out := Run(context.Background(), opts)
	if out.Skipped {
		t.Fatalf("should not skip: %+v", out)
	}
	if out.Kind != "question" {
		t.Errorf("kind = %q, want question", out.Kind)
	}
	if !strings.Contains(out.TaskInfo, "继续部署吗?") {
		t.Errorf("taskInfo = %q", out.TaskInfo)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.requests) != 1 {
		t.Fatalf("gotify requests = %d", len(rec.requests))
	}
	req := rec.requests[0]
	if req.Priority != 6 {
		t.Errorf("question priority = %d, want 6", req.Priority)
	}
	if !strings.Contains(req.Title, "需要你回答") {
		t.Errorf("title = %q", req.Title)
	}
	if !strings.Contains(req.Message, "等待回答于:") {
		t.Errorf("message = %q", req.Message)
	}
	if !strings.Contains(req.Message, "结果: 继续部署吗?") {
		t.Errorf("message missing 结果: %q", req.Message)
	}
}

func TestNotifyHookError(t *testing.T) {
	setupTestEnv(t)
	rec := newGotifyRecorder(t)
	writeConfig(t, rec.server.URL, true)

	payload := `{"hook_source":"opencode-plugin","hook_event_name":"session.error","cwd":"/app","error_message":"segfault","session_id":"s2"}`
	opts := Options{Source: "opencode", FromHook: true, Stdin: strings.NewReader(payload)}

	out := Run(context.Background(), opts)
	if out.Skipped {
		t.Fatalf("should not skip: %+v", out)
	}
	if out.Kind != "error" {
		t.Errorf("kind = %q", out.Kind)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.requests) != 1 {
		t.Fatalf("requests = %d", len(rec.requests))
	}
	if rec.requests[0].Priority != 10 {
		t.Errorf("error priority = %d", rec.requests[0].Priority)
	}
}

func TestNotifyUnknownEventSkipped(t *testing.T) {
	setupTestEnv(t)
	rec := newGotifyRecorder(t)
	writeConfig(t, rec.server.URL, true)

	payload := `{"hook_source":"opencode-plugin","hook_event_name":"session.start","cwd":"/app"}`
	out := Run(context.Background(), Options{Source: "opencode", FromHook: true, Stdin: strings.NewReader(payload)})
	if !out.Skipped {
		t.Fatalf("expected skip, got %+v", out)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.requests) != 0 {
		t.Errorf("no gotify request expected for skip")
	}
}

func TestNotifyDedupe(t *testing.T) {
	setupTestEnv(t)
	rec := newGotifyRecorder(t)
	writeConfig(t, rec.server.URL, true)

	payload := `{"hook_source":"opencode-plugin","hook_event_name":"session.idle","cwd":"/app","session_id":"s3","output_content":"same text"}`
	payload2 := `{"hook_source":"opencode-plugin","hook_event_name":"session.idle","cwd":"/app","session_id":"s4","output_content":"same text"}`

	first := Run(context.Background(), Options{Source: "opencode", FromHook: true, Stdin: strings.NewReader(payload)})
	if first.Skipped {
		t.Fatalf("first should send: %+v", first)
	}
	second := Run(context.Background(), Options{Source: "opencode", FromHook: true, Stdin: strings.NewReader(payload2)})
	if !second.Skipped {
		t.Fatalf("second should be deduped: %+v", second)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.requests) != 1 {
		t.Errorf("expected 1 gotify request after dedupe, got %d", len(rec.requests))
	}
}

func TestNotifySkipDedupe(t *testing.T) {
	setupTestEnv(t)
	rec := newGotifyRecorder(t)
	writeConfig(t, rec.server.URL, true)

	payload := `{"hook_source":"opencode-plugin","hook_event_name":"session.idle","cwd":"/app","session_id":"s5","output_content":"text"}`
	first := Run(context.Background(), Options{Source: "opencode", FromHook: true, Stdin: strings.NewReader(payload)})
	if first.Skipped {
		t.Fatalf("first should send: %+v", first)
	}
	second := Run(context.Background(), Options{Source: "opencode", FromHook: true, SkipDedupe: true, Stdin: strings.NewReader(payload)})
	if second.Skipped {
		t.Fatalf("second with skip-dedupe should send: %+v", second)
	}
}

func TestNotifyDisabled(t *testing.T) {
	setupTestEnv(t)
	rec := newGotifyRecorder(t)
	cfg := config.Default()
	cfg.Gotify.URL = rec.server.URL
	cfg.OpenCode.Enabled = false
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	out := Run(context.Background(), Options{Source: "opencode", TaskInfo: "x"})
	if !out.Skipped {
		t.Fatalf("expected skip when disabled: %+v", out)
	}
}

func TestNotifyDurationThreshold(t *testing.T) {
	setupTestEnv(t)
	rec := newGotifyRecorder(t)
	cfg := config.Default()
	cfg.Gotify.URL = rec.server.URL
	cfg.OpenCode.MinDurationMinutes = 1 // require >= 1 minute
	cfg.Sound.Enabled = false
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	short := int64(10_000) // 10s < 1m
	out := Run(context.Background(), Options{Source: "opencode", TaskInfo: "x", DurationMs: &short})
	if !out.Skipped {
		t.Fatalf("short task should skip: %+v", out)
	}

	long := int64(120_000) // 2m >= 1m
	out = Run(context.Background(), Options{Source: "opencode", TaskInfo: "x", DurationMs: &long})
	if out.Skipped {
		t.Fatalf("long task should send: %+v", out)
	}
}

func TestNotifyForceSendsTestTask(t *testing.T) {
	setupTestEnv(t)
	rec := newGotifyRecorder(t)
	writeConfig(t, rec.server.URL, true)

	out := Run(context.Background(), Options{Source: "opencode", TaskInfo: "测试通知", Force: true, SkipDedupe: true})
	if out.Skipped {
		t.Fatalf("test notify should send: %+v", out)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.requests) != 1 {
		t.Fatalf("requests = %d", len(rec.requests))
	}
}

func TestMissingStdinPayloadSkips(t *testing.T) {
	setupTestEnv(t)
	rec := newGotifyRecorder(t)
	writeConfig(t, rec.server.URL, true)

	// empty stdin -> no payload -> skip
	opts := Options{Source: "opencode", FromHook: true, Stdin: strings.NewReader("")}
	out := Run(context.Background(), opts)
	if !out.Skipped {
		t.Fatalf("empty payload should skip: %+v", out)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.requests) != 0 {
		t.Errorf("no requests expected")
	}
}

func TestGotifyServerFailure(t *testing.T) {
	setupTestEnv(t)
	// server that returns 500
	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer failServer.Close()
	writeConfig(t, failServer.URL, true)

	out := Run(context.Background(), Options{Source: "opencode", TaskInfo: "x", Force: true, SkipDedupe: true})
	if out.OK {
		t.Fatalf("gotify failure should make outcome fail: %+v", out)
	}
	if len(out.Results) != 1 || out.Results[0].OK {
		t.Fatalf("results = %+v", out.Results)
	}
	if out.Results[0].Status != 500 {
		t.Errorf("status = %d", out.Results[0].Status)
	}
}
