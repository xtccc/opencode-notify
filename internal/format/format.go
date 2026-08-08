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
