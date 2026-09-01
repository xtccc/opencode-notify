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

// SentenceBreakChinese inserts a double newline after each Chinese sentence
// terminator "。"、"！"、"？" when not already followed by a double newline.
// Content inside fenced code blocks (```...```) and inline code (`...`) is
// left untouched to avoid breaking markdown/code.
func SentenceBreakChinese(text string) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	parts := strings.Split(text, "```")
	for i := range parts {
		if i%2 == 0 {
			parts[i] = breakChineseOutsideCode(parts[i])
		}
	}
	res := strings.Join(parts, "```")
	// Remove trailing double newline added at overall end to avoid extra blank line before next section.
	if strings.HasSuffix(res, "\n\n") && !strings.HasSuffix(text, "\n\n") {
		res = strings.TrimSuffix(res, "\n\n")
		if strings.HasSuffix(res, "\n") && !strings.HasSuffix(text, "\n") {
			res = strings.TrimSuffix(res, "\n")
		}
	} else if strings.HasSuffix(res, "\n") && !strings.HasSuffix(text, "\n") {
		res = strings.TrimSuffix(res, "\n")
	}
	return res
}

func breakChineseOutsideCode(s string) string {
	// Protect inline code `...` by splitting on backtick.
	inlineParts := strings.Split(s, "`")
	for i := range inlineParts {
		if i%2 == 0 {
			inlineParts[i] = breakChinesePunct(inlineParts[i])
		} else {
			// keep inline code as-is, re-add backticks via Join
		}
	}
	return strings.Join(inlineParts, "`")
}

func breakChinesePunct(s string) string {
	// Insert "\n\n" after each Chinese terminator, skipping spaces and handling existing newlines.
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		b.WriteRune(r)
		if r == '。' || r == '！' || r == '？' {
			// Look ahead, skip spaces
			j := i + 1
			for j < len(runes) && runes[j] == ' ' {
				j++
			}
			if j >= len(runes) {
				// End after spaces - add double newline (will be trimmed if at overall end)
				b.WriteString("\n\n")
				break
			}
			if runes[j] == '\n' {
				// Count existing newlines
				k := j
				cnt := 0
				for k < len(runes) && runes[k] == '\n' {
					cnt++
					k++
				}
				if cnt >= 2 {
					// Already double newline, let next iterations write them
					i = j - 1
				} else {
					// Single newline -> make it double
					b.WriteRune('\n')
					i = j - 1
				}
			} else {
				// Next is not newline - skip spaces and add double newline
				b.WriteString("\n\n")
				i = j - 1
			}
		}
	}
	res := b.String()
	// Normalize triple+ newlines to double
	for strings.Contains(res, "\n\n\n") {
		res = strings.ReplaceAll(res, "\n\n\n", "\n\n")
	}
	return res
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
