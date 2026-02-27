package alarm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	mw "iron/internal/middleware"
	"iron/internal/nlu"
)

func init() {
	registerIntents()
	mw.Register(AlarmDeterministic{})
	mw.Register(AlarmMode{})
	mw.Register(AlarmExec{})
}

/*
Policy:
- confidence < 50  : do nothing
- 50..79           : expose tool alarm.set, LLM decides (tool_choice auto)
- >= 80            : deterministic (cancel; do not call LLM)
*/

const (
	alarmLow  = 50
	alarmHigh = 80
)

const (
	ctxNLUIntent     = "nlu_intent"
	ctxNLUConfidence = "nlu_confidence"
	ctxNLUSlots      = "nlu_slots"
)

func registerIntents() {
	engine := nlu.GetEngine()
	engine.RegisterIntent(
		"set_alarm",
		"set alarm for {time}",
		"set an alarm for {time}",
		"wake me up at {time}",
		"wake me up {time}",
		"create alarm for {time}",
		"alarm at {time}",
		"set alarm at {time}",
	)
}

func ensureContext(e *mw.Event) {
	if e.Context == nil {
		e.Context = map[string]any{}
	}
}

func getAlarmNLU(e *mw.Event) (intent string, confidence int, slots map[string]string) {
	if e == nil {
		return "", 0, nil
	}
	ensureContext(e)
	if v, ok := e.Context[ctxNLUIntent].(string); ok && v != "" {
		intent = v
	}
	if v, ok := e.Context[ctxNLUConfidence].(int); ok {
		confidence = v
	}
	if v, ok := e.Context[ctxNLUSlots].(map[string]string); ok {
		slots = v
	}
	if intent != "" || confidence != 0 || slots != nil {
		return intent, confidence, slots
	}

	parsed := nlu.GetEngine().Parse(e.UserText)
	if parsed.Intent == "set_alarm" && parsed.Confidence > 0 {
		intent = "set_alarm"
		confidence = 100
		slots = parsed.Slots
		e.Context[ctxNLUIntent] = intent
		e.Context[ctxNLUConfidence] = confidence
		e.Context[ctxNLUSlots] = slots
		return intent, confidence, slots
	}

	intent, confidence, slots = alarmHeuristic(e.UserText)
	e.Context[ctxNLUIntent] = intent
	e.Context[ctxNLUConfidence] = confidence
	e.Context[ctxNLUSlots] = slots
	return intent, confidence, slots
}

var (
	reTimeHHMM = regexp.MustCompile(`(?i)\b([01]?\d|2[0-3]):[0-5]\d\b`)
	reTimeAmPm = regexp.MustCompile(`(?i)\b(1[0-2]|0?[1-9])\s*(am|pm)\b`)
	reAfterAt  = regexp.MustCompile(`(?i)\b(?:at|for)\s+(.+)$`)
)

func alarmHeuristic(input string) (intent string, confidence int, slots map[string]string) {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return "", 0, nil
	}

	alarmLike := strings.Contains(s, "alarm") ||
		strings.Contains(s, "wake me up") ||
		strings.Contains(s, "set an alarm") ||
		strings.Contains(s, "set alarm") ||
		strings.Contains(s, "alarme") ||
		strings.Contains(s, "acorda") ||
		strings.Contains(s, "despert")

	if !alarmLike {
		return "", 0, nil
	}

	intent = "set_alarm"
	confidence = 60
	slots = map[string]string{}

	if m := reTimeHHMM.FindStringSubmatch(input); len(m) >= 1 {
		slots["time"] = m[0]
		return intent, confidence, slots
	}
	if m := reTimeAmPm.FindStringSubmatch(input); len(m) >= 3 {
		slots["time"] = strings.TrimSpace(m[1] + m[2])
		return intent, confidence, slots
	}
	if m := reAfterAt.FindStringSubmatch(input); len(m) >= 2 {
		slots["time"] = strings.TrimSpace(m[1])
	}

	if len(slots) == 0 {
		slots = nil
	}
	return intent, confidence, slots
}

/* --------------------- AlarmDeterministic (before_llm_request) --------------------- */

type AlarmDeterministic struct{}

func (AlarmDeterministic) ID() string    { return "alarm_deterministic" }
func (AlarmDeterministic) Priority() int { return 110 }

func (AlarmDeterministic) ShouldLoad(_ context.Context, e *mw.Event) bool {
	return e != nil && e.Name == mw.EventBeforeLLMRequest
}

func (AlarmDeterministic) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventBeforeLLMRequest {
		return mw.Decision{}, nil
	}

	intent, conf, slots := getAlarmNLU(e)
	if intent != "set_alarm" || conf < alarmHigh {
		return mw.Decision{}, nil
	}

	timeStr := "unknown"
	if slots != nil && strings.TrimSpace(slots["time"]) != "" {
		timeStr = slots["time"]
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

	alarms = append(alarms, StoredAlarm{Time: timeStr})
	newData, _ := json.MarshalIndent(alarms, "", "  ")
	os.WriteFile(alarmPath, newData, 0644)

	resp := map[string]any{
		"action":  "set_alarm",
		"time":    timeStr,
		"status":  "success",
		"message": fmt.Sprintf("Alarm set for %s (persisted).", timeStr),
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return mw.Decision{}, err
	}
	s := string(b)

	return mw.Decision{
		Cancel:      true,
		ReplaceText: &s,
		Reason:      "alarm_deterministic: high-confidence; handled locally",
	}, nil
}

/* --------------------------- AlarmMode (filter) --------------------------- */

type AlarmMode struct{}

func (AlarmMode) ID() string    { return "alarm_mode" }
func (AlarmMode) Priority() int { return 110 }

func (AlarmMode) ShouldLoad(_ context.Context, e *mw.Event) bool {
	return e != nil && e.Name == mw.EventBeforeLLMRequest
}

func (AlarmMode) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventBeforeLLMRequest {
		return mw.Decision{}, nil
	}

	params := &mw.LLMParams{}
	if e.Params != nil {
		*params = *e.Params
	}

	params.Tools = upsertTool(params.Tools, AlarmSetTool())
	params.Tools = upsertTool(params.Tools, TimerSetTool())

	intent, conf, _ := getAlarmNLU(e)
	if intent != "set_alarm" || conf < alarmLow || conf >= alarmHigh {
		// Even if not triggered by NLU, we provide the tool to the LLM
		// but we don't return an "OverrideParams" decision unless we want
		// to force tool usage or modify current params.
		// Actually, we should always return it to ensure the tool is there.
		return mw.Decision{
			OverrideParams: params,
			Reason:         "alarm_mode: providing tool to LLM",
		}, nil
	}

	if params.ToolChoice == nil {
		params.ToolChoice = "auto"
	}

	return mw.Decision{
		OverrideParams: params,
		Reason:         "alarm_mode: mid-confidence; tool enabled; LLM decides",
	}, nil
}
