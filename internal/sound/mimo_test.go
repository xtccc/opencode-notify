package sound

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"opencode-notify/internal/config"
)

// fakeWav is an arbitrary byte blob used as the "audio" payload.
var fakeWav = []byte("RIFF\x24\x86\x01\x00WAVEfmt fake-wav-data")

// mimoTestServer returns a fake Mimo API server that records the received
// request and serves the given base64 audio data with the given status.
func mimoTestServer(t *testing.T, status int, b64 string, capture map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		capture["api-key"] = r.Header.Get("api-key")
		capture["body"] = req
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status >= 200 && status < 300 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"audio":{"data":"` + b64 + `"}}}]}`))
		} else {
			_, _ = w.Write([]byte(`{"error":"boom"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// fakePlayer creates a paplay fake in dir that records argv and copies the
// played file, returning the record/copy paths.
func fakePlayer(t *testing.T, dir string) (record, copied string) {
	t.Helper()
	record = filepath.Join(dir, "called")
	copied = filepath.Join(dir, "played.wav")
	script := "#!/bin/sh\necho \"$*\" > " + record + "\ncp \"$1\" " + copied + "\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "paplay"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return record, copied
}

func TestMimoTTSSuccess(t *testing.T) {
	dir := t.TempDir()
	record, copied := fakePlayer(t, dir)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	ResetCache()

	capture := map[string]any{}
	b64 := base64.StdEncoding.EncodeToString(fakeWav)
	srv := mimoTestServer(t, 200, b64, capture)
	mimoEndpointOverride = srv.URL
	t.Cleanup(func() { mimoEndpointOverride = "" })

	cfg := config.SoundConfig{Enabled: true, TTS: true, MimoAPIKey: "secret-key", FallbackBeep: false}
	res := Play(context.Background(), cfg, "你好，任务完成了")
	if !res.OK {
		t.Fatalf("mimo tts should succeed: %+v", res)
	}
	if res.Mode != "tts" {
		t.Errorf("mode = %q", res.Mode)
	}
	if res.Provider != "paplay" {
		t.Errorf("provider = %q", res.Provider)
	}

	if capture["api-key"] != "secret-key" {
		t.Errorf("api-key header = %q", capture["api-key"])
	}
	reqBody, ok := capture["body"].(map[string]any)
	if !ok {
		t.Fatalf("request body not captured: %v", capture["body"])
	}
	if reqBody["model"] != "mimo-v2.5-tts" {
		t.Errorf("model = %v", reqBody["model"])
	}
	audio, _ := reqBody["audio"].(map[string]any)
	if audio["voice"] != "Chloe" {
		t.Errorf("voice = %v", audio["voice"])
	}

	argv, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("read player record: %v", err)
	}
	playedPath := strings.TrimSpace(string(argv))
	if !strings.HasSuffix(playedPath, ".wav") || !strings.Contains(playedPath, "opencode-notify-tts-") {
		t.Errorf("played path = %q", playedPath)
	}
	if _, err := os.Stat(playedPath); !os.IsNotExist(err) {
		t.Errorf("temp file not cleaned up: %s", playedPath)
	}
	got, err := os.ReadFile(copied)
	if err != nil {
		t.Fatalf("read copied wav: %v", err)
	}
	if string(got) != string(fakeWav) {
		t.Errorf("played bytes mismatch: got %d bytes, want %d", len(got), len(fakeWav))
	}
}

func TestMimoTTSMissingKey(t *testing.T) {
	ResetCache()
	cfg := config.SoundConfig{Enabled: true, TTS: true, FallbackBeep: false}
	res := Play(context.Background(), cfg, "x")
	if res.OK {
		t.Fatal("missing key must fail")
	}
	if !strings.Contains(res.Error, "mimoApiKey") {
		t.Errorf("error = %q", res.Error)
	}
}

func TestMimoTTSServerError(t *testing.T) {
	dir := t.TempDir()
	fakePlayer(t, dir)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	ResetCache()

	capture := map[string]any{}
	srv := mimoTestServer(t, 500, "", capture)
	mimoEndpointOverride = srv.URL
	t.Cleanup(func() { mimoEndpointOverride = "" })

	cfg := config.SoundConfig{Enabled: true, TTS: true, MimoAPIKey: "k", FallbackBeep: false}
	res := Play(context.Background(), cfg, "x")
	if res.OK {
		t.Fatal("500 must fail")
	}
	if !strings.Contains(res.Error, "HTTP 500") {
		t.Errorf("error = %q", res.Error)
	}
}

func TestMimoTTSPlaybackFailure(t *testing.T) {
	dir := t.TempDir()
	// fake paplay that fails
	script := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "paplay"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	ResetCache()

	capture := map[string]any{}
	b64 := base64.StdEncoding.EncodeToString(fakeWav)
	srv := mimoTestServer(t, 200, b64, capture)
	mimoEndpointOverride = srv.URL
	t.Cleanup(func() { mimoEndpointOverride = "" })

	cfg := config.SoundConfig{Enabled: true, TTS: true, MimoAPIKey: "k", FallbackBeep: false}
	res := Play(context.Background(), cfg, "x")
	if res.OK {
		t.Fatal("playback failure must fail")
	}
	if res.Mode != "tts" {
		t.Errorf("mode = %q", res.Mode)
	}
}

func TestMimoTTSUsesStaticText(t *testing.T) {
	dir := t.TempDir()
	fakePlayer(t, dir)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	ResetCache()

	capture := map[string]any{}
	b64 := base64.StdEncoding.EncodeToString(fakeWav)
	srv := mimoTestServer(t, 200, b64, capture)
	mimoEndpointOverride = srv.URL
	t.Cleanup(func() { mimoEndpointOverride = "" })

	cfg := config.SoundConfig{Enabled: true, TTS: true, MimoAPIKey: "k", StaticText: "任务完成", FallbackBeep: false}
	res := Play(context.Background(), cfg, "ignored")
	if !res.OK {
		t.Fatalf("mimo tts should succeed: %+v", res)
	}
	msgs, _ := capture["body"].(map[string]any)["messages"].([]any)
	last := msgs[len(msgs)-1].(map[string]any)
	if last["content"] != "任务完成" {
		t.Errorf("spoken text = %v, want 任务完成", last["content"])
	}
}
