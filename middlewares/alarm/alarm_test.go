package alarm

import (
	"context"
	"strings"
	"testing"

	mw "iron/internal/middleware"
)

func TestAlarmDeterministicCancelsHighConfidence(t *testing.T) {
	m := AlarmDeterministic{}
	ev := &mw.Event{
		Name:     mw.EventBeforeLLMRequest,
		UserText: "set alarm for 07:30",
		Params:   &mw.LLMParams{},
	}
	dec, err := m.OnEvent(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !dec.Cancel || dec.ReplaceText == nil {
		t.Fatalf("expected cancel + ReplaceText")
	}
	if !strings.Contains(*dec.ReplaceText, "07:30") {
		t.Fatalf("expected response to mention time, got: %s", *dec.ReplaceText)
	}
}

func TestAlarmModeInjectsToolMidConfidence(t *testing.T) {
	m := AlarmMode{}
	ev := &mw.Event{
		Name:     mw.EventBeforeLLMRequest,
		UserText: "please set an alarm tomorrow morning",
		Params:   &mw.LLMParams{},
	}
	dec, err := m.OnEvent(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if dec.OverrideParams == nil {
		t.Fatalf("expected OverrideParams")
	}
	found := false
	for _, tool := range dec.OverrideParams.Tools {
		if tool.Function != nil && tool.Function.Name == "alarm.set" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected alarm.set tool injected")
	}
	if dec.OverrideParams.ToolChoice != "auto" {
		t.Fatalf("expected ToolChoice auto")
	}
}

func TestAlarmModeNoopLowConfidence(t *testing.T) {
	m := AlarmMode{}
	ev := &mw.Event{
		Name:     mw.EventBeforeLLMRequest,
		UserText: "hello world",
		Params:   &mw.LLMParams{},
	}
	dec, err := m.OnEvent(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if dec.Cancel || dec.ReplaceText != nil || dec.OverrideParams != nil {
		t.Fatalf("expected no-op decision, got: %+v", dec)
	}
}

func TestAlarmExecHandlesOnlyAlarmSet(t *testing.T) {
	m := AlarmExec{}
	ev := &mw.Event{
		Name: mw.EventAfterLLMResponse,
		Context: map[string]any{
			"tool_calls": []mw.ToolCall{
				{Tool: "calendar.list", Args: map[string]any{"start_date": "2026-02-25"}},
				{Tool: "alarm.set", Args: map[string]any{"time": "07:30", "label": "wake"}},
			},
		},
	}
	dec, err := m.OnEvent(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !dec.Cancel || dec.ReplaceText == nil {
		t.Fatalf("expected cancel + ReplaceText")
	}
	if !strings.Contains(*dec.ReplaceText, "07:30") {
		t.Fatalf("expected output to mention time, got: %s", *dec.ReplaceText)
	}
}
