package alarm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	mw "iron/internal/middleware"
)

// AlarmExec executes alarm.set tool calls emitted by the model.
// It runs on after_llm_response and cancels further processing when it handles
// at least one alarm tool call.
type AlarmExec struct{}

func (AlarmExec) ID() string    { return "alarm_exec" }
func (AlarmExec) Priority() int { return 70 }

func (AlarmExec) ShouldLoad(_ context.Context, e *mw.Event) bool {
	if e == nil || e.Context == nil {
		return false
	}
	_, ok := e.Context["tool_calls"].([]mw.ToolCall)
	return ok
}

func (AlarmExec) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventAfterLLMResponse {
		return mw.Decision{}, nil
	}

	raw, ok := e.Context["tool_calls"].([]mw.ToolCall)
	if !ok || len(raw) == 0 {
		return mw.Decision{}, nil
	}

	outputs := make([]string, 0, len(raw))
	handled := false
	for _, tc := range raw {
		if tc.Tool == "alarm.set" {
			handled = true
			out := runAlarmTool(tc)
			if strings.TrimSpace(out) != "" {
				outputs = append(outputs, out)
			}
		} else if tc.Tool == "timer.set" {
			handled = true
			out := runTimerTool(tc)
			if strings.TrimSpace(out) != "" {
				outputs = append(outputs, out)
			}
		}
	}
	if !handled {
		return mw.Decision{}, nil
	}

	text := strings.Join(outputs, "\n\n")
	return mw.Decision{
		Cancel:      true,
		ReplaceText: &text,
		Reason:      "alarm_exec",
	}, nil
}

type StoredAlarm struct {
	Time  string `json:"time"`
	Label string `json:"label,omitempty"`
}

func runAlarmTool(tc mw.ToolCall) string {
	timeStr, _ := tc.Args["time"].(string)
	label, _ := tc.Args["label"].(string)

	if strings.TrimSpace(timeStr) == "" {
		return `alarm.set: missing required arg "time"`
	}

	// Persist to ~/.iron/alarms.json
	home, _ := os.UserHomeDir()
	alarmPath := filepath.Join(home, ".iron", "alarms.json")
	os.MkdirAll(filepath.Dir(alarmPath), 0755)

	var alarms []StoredAlarm
	data, err := os.ReadFile(alarmPath)
	if err == nil {
		json.Unmarshal(data, &alarms)
	}

	alarms = append(alarms, StoredAlarm{Time: timeStr, Label: label})
	newData, _ := json.MarshalIndent(alarms, "", "  ")
	os.WriteFile(alarmPath, newData, 0644)

	if strings.TrimSpace(label) == "" {
		return fmt.Sprintf("ok: alarm set for %s (persisted)", timeStr)
	}
	return fmt.Sprintf("ok: alarm set for %s (%s) (persisted)", timeStr, label)
}

func runTimerTool(tc mw.ToolCall) string {
	minutes, ok := tc.Args["minutes"].(float64)
	if !ok || minutes <= 0 {
		return "timer.set: invalid or missing 'minutes' argument"
	}
	message, _ := tc.Args["message"].(string)
	if message == "" {
		message = "Timer finished!"
	}

	seconds := int(minutes * 60)
	var cmd *exec.Cmd

	if runtime.GOOS == "darwin" {
		shCmd := fmt.Sprintf(`sleep %d && osascript -e 'display notification "%s" with title "IRon Timer"'`, seconds, message)
		cmd = exec.Command("sh", "-c", shCmd)
	} else if runtime.GOOS == "linux" {
		shCmd := fmt.Sprintf(`sleep %d && notify-send "IRon Timer" "%s"`, seconds, message)
		cmd = exec.Command("sh", "-c", shCmd)
	} else {
		return "timer.set: background timers are currently only supported on Linux and macOS"
	}

	// Start detaches the process, allowing IRon to exit while the timer runs
	if err := cmd.Start(); err != nil {
		return fmt.Sprintf("timer.set failed to start: %v", err)
	}

	// We intentionally do not call cmd.Wait() so it becomes an orphan/background process

	return fmt.Sprintf("ok: timer set for %.1f minutes with message: '%s'", minutes, message)
}
