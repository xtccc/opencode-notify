// Package hookcontext parses the stdin JSON payload produced by the
// generated opencode plugin into a notification decision.
package hookcontext

import (
	"encoding/json"
	"io"
	"strings"
	"time"
)

// Kind describes the notification type.
type Kind string

const (
	KindComplete Kind = "complete"
	KindError    Kind = "error"
	KindQuestion Kind = "question"
)

// Known event names emitted by the opencode plugin.
const (
	EventSessionIdle      = "session.idle"
	EventSessionError     = "session.error"
	EventSessionStatus    = "session.status"
	EventQuestionAsked    = "question.asked"
	EventQuestionV2Asked  = "question.v2.asked"
)

// HookPayload is the JSON contract piped from the opencode plugin.
type HookPayload struct {
	HookSource       string `json:"hook_source"`
	HookEventName    string `json:"hook_event_name"`
	Cwd              string `json:"cwd"`
	TaskInfo         string `json:"task_info"`
	SessionID        string `json:"session_id"`
	ProjectName      string `json:"project_name"`
	ErrorMessage     string `json:"error_message"`
	AssistantMessage string `json:"assistant_message"`
	OutputContent    string `json:"output_content"`
	QuestionText     string `json:"question_text"`
}

// Decision is the result of parsing a payload: either a notification to
// send or a skip.
type Decision struct {
	Kind        Kind
	TaskInfo    string
	OutputText  string
	Cwd         string
	ProjectName string
	Signature   string // dedupe signature input
	Skip        bool
	SkipReason  string
}

// ReadStdinJSON reads a JSON object from r with a short timeout. It never
// blocks past deadline and returns nil on empty/invalid input.
func ReadStdinJSON(r io.Reader, timeout time.Duration) (*HookPayload, error) {
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}

	type result struct {
		payload *HookPayload
		err     error
	}

	ch := make(chan result, 1)
	go func() {
		data, err := io.ReadAll(r)
		if err != nil {
			ch <- result{nil, err}
			return
		}
		trimmed := strings.TrimSpace(string(data))
		if trimmed == "" {
			ch <- result{nil, nil}
			return
		}
		var p HookPayload
		if err := json.Unmarshal([]byte(trimmed), &p); err != nil {
			ch <- result{nil, nil}
			return
		}
		ch <- result{&p, nil}
	}()

	select {
	case res := <-ch:
		return res.payload, res.err
	case <-time.After(timeout):
		return nil, nil
	}
}

// BuildDecision maps a payload to a notification Decision. The optional
// explicitTaskInfo (from --task) overrides the default task labels.
func Build(payload *HookPayload, explicitTaskInfo string) (*Decision, error) {
	if payload == nil {
		return &Decision{Skip: true, SkipReason: "empty hook payload"}, nil
	}
	if payload.HookSource != "" && payload.HookSource != "opencode-plugin" {
		return &Decision{Skip: true, SkipReason: "unexpected hook_source: " + payload.HookSource}, nil
	}

	eventName := strings.TrimSpace(payload.HookEventName)
	if eventName == "" {
		return &Decision{Skip: true, SkipReason: "missing hook_event_name"}, nil
	}

	output := firstNonEmpty(payload.OutputContent, payload.AssistantMessage, payload.ErrorMessage)
	if eventName == EventSessionError {
		failure := truncate(firstNonEmpty(payload.ErrorMessage, output, "OpenCode task failed"), 88)
		return &Decision{
			Kind:        KindError,
			TaskInfo:    explicitOrDefault(explicitTaskInfo, "OpenCode 失败: "+failure),
			OutputText:  output,
			Cwd:         payload.Cwd,
			ProjectName: payload.ProjectName,
			Signature:   output,
		}, nil
	}

	if eventName == EventQuestionAsked || eventName == EventQuestionV2Asked {
		question := truncate(firstNonEmpty(payload.QuestionText, output), 88)
		task := "OpenCode 需要你回答"
		if question != "" {
			task += ": " + question
		}
		return &Decision{
			Kind:        KindQuestion,
			TaskInfo:    explicitOrDefault(explicitTaskInfo, task),
			OutputText:  firstNonEmpty(payload.QuestionText, output),
			Cwd:         payload.Cwd,
			ProjectName: payload.ProjectName,
			Signature:   firstNonEmpty(payload.QuestionText, output),
		}, nil
	}

	if eventName != EventSessionIdle && eventName != EventSessionStatus {
		return &Decision{Skip: true, SkipReason: "unsupported event: " + eventName}, nil
	}

	return &Decision{
		Kind:        KindComplete,
		TaskInfo:    explicitOrDefault(explicitTaskInfo, "OpenCode 完成"),
		OutputText:  output,
		Cwd:         payload.Cwd,
		ProjectName: payload.ProjectName,
		Signature:   output,
	}, nil
}

func explicitOrDefault(explicit, fallback string) string {
	if v := strings.TrimSpace(explicit); v != "" && v != "任务已完成" {
		return v
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func truncate(text string, max int) string {
	value := strings.TrimSpace(text)
	if value == "" || len(value) <= max {
		return value
	}
	return strings.TrimSpace(value[:max-3]) + "..."
}
