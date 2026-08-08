package gotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"opencode-notify/internal/config"
)

func TestSendOK(t *testing.T) {
	var gotPath, gotKey string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("X-Gotify-Key")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config.GotifyConfig{
		URL:       srv.URL,
		AppToken:  "sekret",
		TimeoutMs: 5000,
		Priority:  config.DefaultPriority(),
	}
	res := Send(context.Background(), cfg, "hello", "body", KindComplete)
	if !res.OK || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %+v", res)
	}
	if gotPath != "/message" {
		t.Errorf("path = %q, want /message", gotPath)
	}
	if gotKey != "sekret" {
		t.Errorf("X-Gotify-Key = %q", gotKey)
	}
	if gotBody["title"] != "hello" || gotBody["message"] != "body" {
		t.Errorf("unexpected body: %v", gotBody)
	}
	if gotBody["priority"] != float64(5) {
		t.Errorf("priority = %v, want 5", gotBody["priority"])
	}
}

func TestSendErrorPriority(t *testing.T) {
	var pri float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		pri, _ = b["priority"].(float64)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config.GotifyConfig{URL: srv.URL, AppToken: "t", TimeoutMs: 5000, Priority: config.DefaultPriority()}
	res := Send(context.Background(), cfg, "t", "m", KindError)
	if !res.OK {
		t.Fatalf("unexpected result: %+v", res)
	}
	if pri != 10 {
		t.Errorf("error priority = %v, want 10", pri)
	}
}

func TestSendHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	cfg := config.GotifyConfig{URL: srv.URL, AppToken: "t", TimeoutMs: 5000, Priority: config.DefaultPriority()}
	res := Send(context.Background(), cfg, "t", "m", KindComplete)
	if res.OK {
		t.Fatal("expected failure")
	}
	if res.Status != 500 {
		t.Errorf("status = %d", res.Status)
	}
	if !strings.Contains(res.Error, "HTTP 500") {
		t.Errorf("error = %q", res.Error)
	}
}

func TestSendMissingConfig(t *testing.T) {
	if res := Send(context.Background(), config.GotifyConfig{}, "t", "m", KindComplete); res.OK {
		t.Fatal("empty config should fail")
	}
	cfg := config.GotifyConfig{URL: "http://x.in"}
	if res := Send(context.Background(), cfg, "t", "m", KindComplete); res.OK {
		t.Fatal("missing token should fail")
	}
}

func TestSendInvalidURL(t *testing.T) {
	cfg := config.GotifyConfig{URL: ":://bad::", AppToken: "t"}
	if res := Send(context.Background(), cfg, "t", "m", KindComplete); res.OK {
		t.Fatal("invalid url should fail")
	}
}

func TestRedact(t *testing.T) {
	in := "error at https://gotify.example.com/message?token=abc123&secret=hush keep-me"
	out := Redact(in)
	if strings.Contains(out, "abc123") || strings.Contains(out, "hush") || strings.Contains(out, "https://gotify") {
		t.Fatalf("redact failed: %q", out)
	}
	if !strings.Contains(out, "keep-me") {
		t.Fatalf("redact removed too much: %q", out)
	}
}
