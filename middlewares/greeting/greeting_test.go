package greeting

import (
	"context"
	"testing"

	mw "iron/internal/middleware"
)

func TestGreetingCancelsLLM(t *testing.T) {
	g := Greeting{}
	ev := &mw.Event{
		Name:     mw.EventBeforeLLMRequest,
		UserText: "Hello!",
	}
	dec, err := g.OnEvent(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !dec.Cancel {
		t.Fatalf("expected cancel for greeting")
	}
	if dec.ReplaceText == nil || *dec.ReplaceText == "" {
		t.Fatalf("expected replacement text")
	}
}

func TestGreetingSkipsNonGreeting(t *testing.T) {
	g := Greeting{}
	ev := &mw.Event{
		Name:     mw.EventBeforeLLMRequest,
		UserText: "hello please summarize this doc",
	}
	dec, err := g.OnEvent(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if dec.Cancel {
		t.Fatalf("should not cancel non-greeting")
	}
}
