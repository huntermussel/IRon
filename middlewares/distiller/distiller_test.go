package distiller

import (
	"context"
	"strings"
	"testing"

	mw "iron/internal/middleware"
	"iron/internal/memory"
)

func TestDistiller_ID_Priority(t *testing.T) {
	d := New()
	if d.ID() != "distiller" {
		t.Errorf("unexpected ID: %s", d.ID())
	}
	if d.Priority() != 105 {
		t.Errorf("unexpected priority: %d", d.Priority())
	}
}

func TestDistiller_ShouldLoad_Default(t *testing.T) {
	d := New()
	e := &mw.Event{Context: nil}
	if !d.ShouldLoad(context.Background(), e) {
		t.Error("expected ShouldLoad=true by default")
	}
}

func TestDistiller_ShouldLoad_OptOut(t *testing.T) {
	d := New()
	e := &mw.Event{Context: map[string]any{"distiller": false}}
	if d.ShouldLoad(context.Background(), e) {
		t.Error("expected ShouldLoad=false when context['distiller']=false")
	}
}

func TestDistiller_BeforeRequest_NoContext_NoChange(t *testing.T) {
	d := New()
	// Fresh distiller with no accumulated key points → no enrichment
	e := &mw.Event{
		Name:     mw.EventBeforeLLMRequest,
		UserText: "what is the capital of France",
		Context:  map[string]any{},
	}
	dec, err := d.OnEvent(context.Background(), e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Cancel {
		t.Error("should not cancel when no context accumulated")
	}
	if dec.ReplaceText != nil {
		t.Errorf("should not replace text when nothing to inject, got: %s", *dec.ReplaceText)
	}
}

func TestDistiller_BeforeRequest_InjectsContext(t *testing.T) {
	d := New()

	// Seed some key points directly
	d.kps.Upsert(memory.KeyPoint{Type: memory.KPFact, Key: "lang", Value: "go", Session: "default"})
	d.kps.Upsert(memory.KeyPoint{Type: memory.KPPref, Key: "no_frameworks", Session: "default"})

	e := &mw.Event{
		Name:     mw.EventBeforeLLMRequest,
		UserText: "how do I structure this project",
		Context:  map[string]any{},
	}
	dec, err := d.OnEvent(context.Background(), e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.ReplaceText == nil {
		t.Fatal("expected ReplaceText with DSL context")
	}
	if !strings.Contains(*dec.ReplaceText, "[CTX:") {
		t.Errorf("expected [CTX:...] header, got: %s", *dec.ReplaceText)
	}
	if !strings.Contains(*dec.ReplaceText, "lang=go") {
		t.Errorf("expected lang=go in context, got: %s", *dec.ReplaceText)
	}
}

func TestDistiller_AfterResponse_ExtractsKeyPoints(t *testing.T) {
	d := New()

	e := &mw.Event{
		Name:     mw.EventAfterLLMResponse,
		UserText: "I prefer Go and my database is postgres",
		LLMText:  "Great, I will use Go with a Postgres backend for this project setup.",
		Context:  map[string]any{},
	}
	_, err := d.OnEvent(context.Background(), e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dsl := d.kps.FormatDSL("default")
	if !strings.Contains(dsl, "go") && !strings.Contains(dsl, "postgres") {
		t.Errorf("expected extracted key points in DSL, got: %s", dsl)
	}
}

func TestDistiller_SemanticCache_HitAfterAfterResponse(t *testing.T) {
	d := New()

	// Populate cache via afterResponse
	afterEv := &mw.Event{
		Name:     mw.EventAfterLLMResponse,
		UserText: "how do I run tests for this go module project",
		LLMText:  "run go test -race ./... to execute all tests with race detection enabled",
		Context:  map[string]any{},
	}
	d.OnEvent(context.Background(), afterEv) //nolint:errcheck

	// Now a similar query should hit the cache
	beforeEv := &mw.Event{
		Name:     mw.EventBeforeLLMRequest,
		UserText: "how do I run tests for this go module project",
		Context:  map[string]any{},
	}
	dec, err := d.OnEvent(context.Background(), beforeEv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dec.Cancel {
		t.Error("expected Cancel=true for semantic cache hit")
	}
	if dec.ReplaceText == nil || *dec.ReplaceText == "" {
		t.Error("expected cached response in ReplaceText")
	}
}

func TestDistiller_ProcHint_Injected(t *testing.T) {
	d := New()

	e := &mw.Event{
		Name:     mw.EventBeforeLLMRequest,
		UserText: "build my go project and run tests",
		Context:  map[string]any{},
	}
	dec, err := d.OnEvent(context.Background(), e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.ReplaceText == nil {
		t.Fatal("expected ReplaceText with procedure hint")
	}
	if !strings.Contains(*dec.ReplaceText, "[PROC:") {
		t.Errorf("expected [PROC:...] hint, got: %s", *dec.ReplaceText)
	}
}

func TestDistiller_IgnoresNonTargetEvents(t *testing.T) {
	d := New()
	e := &mw.Event{
		Name:     mw.EventBeforeUserReply,
		UserText: "anything",
		Context:  map[string]any{},
	}
	dec, err := d.OnEvent(context.Background(), e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec.Cancel || dec.ReplaceText != nil {
		t.Error("distiller should be a no-op for before_user_reply events")
	}
}
