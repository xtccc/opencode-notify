package format

import (
	"strings"
	"testing"
	"time"
)

func TestFormatDurationMs(t *testing.T) {
	cases := []struct {
		name string
		in   *int64
		want string
	}{
		{"nil", nil, ""},
		{"negative", i64(-5), ""},
		{"seconds only", i64(42_000), "42s"},
		{"minutes and seconds", i64(305_000), "5m 5s"},
		{"exact minute", i64(120_000), "2m 0s"},
		{"zero", i64(0), "0s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatDurationMs(tc.in); got != tc.want {
				t.Errorf("FormatDurationMs(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestBuildTitle(t *testing.T) {
	if got := BuildTitle("my-app", "OpenCode 完成", "OpenCode"); got != "[OpenCode] my-app: OpenCode 完成" {
		t.Errorf("unexpected title: %q", got)
	}
	if got := BuildTitle("", "OpenCode 完成", "OpenCode"); got != "OpenCode 完成" {
		t.Errorf("unexpected title without project: %q", got)
	}
}

func TestSourceLabel(t *testing.T) {
	if got := SourceLabel("opencode"); got != "OpenCode" {
		t.Errorf("SourceLabel(opencode) = %q", got)
	}
	if got := SourceLabel(""); got != "unknown" {
		t.Errorf("SourceLabel() = %q", got)
	}
}

func TestTruncateSummary(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"empty", "", 10, ""},
		{"short unchanged", "hello", 10, "hello"},
		{"multiline flattened", "a   b\n\nc\td", 10, "a b c d"},
		{"truncated", "abcdefghij", 7, "abcd..."},
		{"runes not bytes", "中文摘要内容", 4, "中..."},
		{"max too small", "abcdef", 1, "a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := TruncateSummary(tc.in, tc.max); got != tc.want {
				t.Errorf("TruncateSummary(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}

func TestTimestamp(t *testing.T) {
	got := Timestamp(time.Unix(1_700_000_000, 0))
	if !strings.Contains(got, ":") {
		t.Errorf("unexpected timestamp: %q", got)
	}
}

func i64(v int64) *int64 { return &v }
