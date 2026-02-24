package slack

import (
	"context"
	"testing"

	mw "iron/internal/middleware"
)

func TestSlackToolsInjected(t *testing.T) {
	m := SlackTools{}
	ev := &mw.Event{
		Name:   mw.EventBeforeLLMRequest,
		Params: &mw.LLMParams{},
	}
	dec, err := m.OnEvent(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if dec.OverrideParams == nil {
		t.Fatalf("expected OverrideParams")
	}
	if len(dec.OverrideParams.Tools) < 2 {
		t.Fatalf("expected at least 2 slack tools, got %d", len(dec.OverrideParams.Tools))
	}
}
