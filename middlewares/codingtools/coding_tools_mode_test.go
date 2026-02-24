package codingtools

import (
	"context"
	"testing"

	mw "iron/internal/middleware"
)

func TestCodingToolsInjected(t *testing.T) {
	m := CodingToolsMode{}
	ev := &mw.Event{
		Name: mw.EventBeforeLLMRequest,
		Params: &mw.LLMParams{
			MaxTokens: 1000,
		},
	}
	dec, err := m.OnEvent(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if dec.OverrideParams == nil {
		t.Fatalf("expected OverrideParams")
	}
	if len(dec.OverrideParams.Tools) < 7 {
		t.Fatalf("expected at least 7 tools, got %d", len(dec.OverrideParams.Tools))
	}
	if dec.OverrideParams.ToolChoice != "auto" {
		t.Fatalf("expected ToolChoice auto")
	}
}
