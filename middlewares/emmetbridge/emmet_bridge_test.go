package emmetbridge

import (
	"context"
	"strings"
	"testing"

	mw "iron/internal/middleware"
)

func TestEmmetBridge_HTMLInput(t *testing.T) {
	bridge := EmmetBridge{}
	ev := &mw.Event{
		Name:     mw.EventBeforeLLMRequest,
		UserText: "<div></div>", // looksLikeHTML = true
	}
	dec, err := bridge.OnEvent(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if dec.ReplaceText == nil {
		t.Fatalf("expected ReplaceText for HTML input")
	}
	if !strings.Contains(*dec.ReplaceText, "if HTML respond in Emmet only") {
		t.Errorf("expected instruction 'if HTML respond in Emmet only', got: %s", *dec.ReplaceText)
	}
	// Check if conversion happened
	// htmlToEmmet("<div></div>") -> "div"
	if !strings.Contains(*dec.ReplaceText, "div") {
		t.Errorf("expected converted Emmet 'div' in prompt, got: %s", *dec.ReplaceText)
	}
}

func TestEmmetBridge_NaturalLanguageInput(t *testing.T) {
	bridge := EmmetBridge{}
	ev := &mw.Event{
		Name:     mw.EventBeforeLLMRequest,
		UserText: "make a div", // looksLikeHTML = false (unless very permissive)
	}
	dec, err := bridge.OnEvent(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if dec.ReplaceText == nil {
		t.Fatalf("expected ReplaceText for natural language input (system prompt injection)")
	}
	// It should append the system prompt
	expectedPrompt := " [SYSTEM: If generating HTML, output ONLY Emmet syntax. Do not use standard HTML tags.]"
	if !strings.Contains(*dec.ReplaceText, expectedPrompt) {
		t.Errorf("expected appended system prompt '%s', got: %s", expectedPrompt, *dec.ReplaceText)
	}
	if !strings.Contains(*dec.ReplaceText, "make a div") {
		t.Errorf("original user text should be preserved, got: %s", *dec.ReplaceText)
	}
}

func TestEmmetBridge_EmmetOutput(t *testing.T) {
	bridge := EmmetBridge{}
	ev := &mw.Event{
		Name:    mw.EventAfterLLMResponse,
		LLMText: "div>span", // looksLikeEmmet = true
	}
	dec, err := bridge.OnEvent(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if dec.ReplaceText == nil {
		t.Fatalf("expected ReplaceText for Emmet output")
	}
	// emmetToHTML("div>span") -> "<div><span></span></div>"
	// (Depending on implementation details)
	if !strings.Contains(*dec.ReplaceText, "<div>") {
		t.Errorf("expected HTML conversion containing '<div>', got: %s", *dec.ReplaceText)
	}
}
