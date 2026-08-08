package hookcontext

import (
	"strings"
	"testing"
	"time"
)

func TestBuild(t *testing.T) {
	cases := []struct {
		name     string
		payload  *HookPayload
		explicit string
		wantKind Kind
		wantTask string
		skip     bool
	}{
		{
			name:     "idle",
			payload:  &HookPayload{HookSource: "opencode-plugin", HookEventName: "session.idle", Cwd: "/s", OutputContent: "done"},
			wantKind: KindComplete, wantTask: "OpenCode 完成",
		},
		{
			name:     "idle explicit task wins",
			payload:  &HookPayload{HookSource: "opencode-plugin", HookEventName: "session.idle", Cwd: "/srv/app"},
			explicit: "我的任务", wantKind: KindComplete, wantTask: "我的任务",
		},
		{
			name:     "error",
			payload:  &HookPayload{HookSource: "opencode-plugin", HookEventName: "session.error", Cwd: "/s", ErrorMessage: "API Error 429: rate limited"},
			wantKind: KindError, wantTask: "OpenCode 失败: API Error 429: rate limited",
		},
		{
			name:     "status is completion",
			payload:  &HookPayload{HookSource: "opencode-plugin", HookEventName: "session.status", Cwd: "/s"},
			wantKind: KindComplete, wantTask: "OpenCode 完成",
		},
		{
			name:    "unknown event skipped",
			payload: &HookPayload{HookSource: "opencode-plugin", HookEventName: "session.started", Cwd: "/s"},
			skip:    true,
		},
		{
			name:    "missing event skipped",
			payload: &HookPayload{HookSource: "opencode-plugin", Cwd: "/s"},
			skip:    true,
		},
		{
			name:    "nil payload skipped",
			payload: nil,
			skip:    true,
		},
		{
			name:    "wrong source skipped",
			payload: &HookPayload{HookSource: "other", HookEventName: "session.idle"},
			skip:    true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := Build(tc.payload, tc.explicit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.Skip != tc.skip {
				t.Errorf("skip = %v, want %v (reason %q)", d.Skip, tc.skip, d.SkipReason)
			}
			if d.Skip {
				return
			}
			if d.Kind != tc.wantKind {
				t.Errorf("kind = %q, want %q", d.Kind, tc.wantKind)
			}
			if d.TaskInfo != tc.wantTask {
				t.Errorf("taskInfo = %q, want %q", d.TaskInfo, tc.wantTask)
			}
		})
	}
}

func TestBuildErrorTruncation(t *testing.T) {
	long := strings.Repeat("x", 200)
	d, _ := Build(&HookPayload{HookSource: "opencode-plugin", HookEventName: "session.error", ErrorMessage: long}, "")
	if len(d.TaskInfo) > len("OpenCode 失败: ")+88 {
		t.Errorf("task info not truncated: %d", len(d.TaskInfo))
	}
}

func TestReadStdinJSON(t *testing.T) {
	payload, err := ReadStdinJSON(strings.NewReader(`{"hook_source":"opencode-plugin","hook_event_name":"session.idle","cwd":"/x"}`), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload == nil || payload.HookEventName != "session.idle" || payload.Cwd != "/x" {
		t.Fatalf("unexpected payload: %+v", payload)
	}

	if p, err := ReadStdinJSON(strings.NewReader(""), 0); err != nil || p != nil {
		t.Fatalf("empty stdin should yield nil payload, got %+v err=%v", p, err)
	}

	if p, err := ReadStdinJSON(strings.NewReader("not json"), 0); err != nil || p != nil {
		t.Fatalf("invalid stdin should yield nil payload, got %+v err=%v", p, err)
	}
}

func TestStdinJSONTimeoutNeverHangs(t *testing.T) {
	blocking := &neverReader{}
	done := make(chan struct{})
	go func() {
		p, err := ReadStdinJSON(blocking, 50*time.Millisecond)
		if err != nil || p != nil {
			t.Errorf("expected nil/nil on timeout, got %+v err=%v", p, err)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ReadStdinJSON blocked past deadline")
	}
}

type neverReader struct{}

func (neverReader) Read([]byte) (int, error) { select {} }
