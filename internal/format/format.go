package format

import (
	"strings"
	"time"
)

// SourceLabel returns the display label for a source name.
func SourceLabel(source string) string {
	switch strings.ToLower(source) {
	case "opencode":
		return "OpenCode"
	default:
		if source == "" {
			return "unknown"
		}
		return source
	}
}

// FormatDurationMs renders a duration in ms as "Xs" or "Xm Ys".
// A nil/negative value returns an empty string.
func FormatDurationMs(durationMs *int64) string {
	if durationMs == nil || *durationMs < 0 {
		return ""
	}
	totalSeconds := *durationMs / 1000
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	if minutes <= 0 {
		return formatI64(seconds) + "s"
	}
	return formatI64(minutes) + "m " + formatI64(seconds) + "s"
}

// BuildTitle composes the notification title:
//   - with project: "[OpenCode] project: task"
//   - without project: task (no prefix)
func BuildTitle(projectName, taskInfo, sourceLabel string) string {
	if projectName == "" {
		return taskInfo
	}
	return "[" + sourceLabel + "] " + projectName + ": " + taskInfo
}

// TruncateSummary flattens a multi-line text into a single line and
// truncates it to at most max runes. Empty input returns "".
func TruncateSummary(text string, max int) string {
	value := strings.Join(strings.Fields(text), " ")
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return string(runes)
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

// Timestamp returns a human readable local timestamp (Asia/Shanghai when
// available, otherwise local time), matching the original zh-CN format.
func Timestamp(now time.Time) string {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.Local
	}
	return now.In(loc).Format("2006-01-02 15:04:05")
}

func formatI64(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
