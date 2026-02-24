package localcache

import (
	"context"
	"testing"
	"time"

	mw "iron/internal/middleware"
)

func TestLocalCache_SkipWait(t *testing.T) {
	lc := &LocalCache{
		cache: make(map[string]cacheEntry),
	}

	ctx := context.Background()

	// 1. First request (Miss)
	req1 := &mw.Event{
		Name:     mw.EventBeforeLLMRequest,
		UserText: "What is the capital of France?",
	}
	dec1, err := lc.OnEvent(ctx, req1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec1.Cancel {
		t.Fatalf("expected miss on first request, got cancel")
	}

	// 2. Response comes back (Store)
	resp1 := &mw.Event{
		Name:     mw.EventAfterLLMResponse,
		UserText: "What is the capital of France?",
		LLMText:  "Paris",
	}
	_, err = lc.OnEvent(ctx, resp1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify cache content directly (internal access)
	entry, ok := lc.cache["What is the capital of France?"]
	if !ok {
		t.Fatalf("expected cache entry to be stored")
	}
	if entry.response != "Paris" {
		t.Fatalf("expected cached response 'Paris', got '%s'", entry.response)
	}

	// 3. Second request (Hit)
	req2 := &mw.Event{
		Name:     mw.EventBeforeLLMRequest,
		UserText: "What is the capital of France?",
	}
	dec2, err := lc.OnEvent(ctx, req2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dec2.Cancel {
		t.Fatalf("expected hit on second request, did not cancel")
	}
	if dec2.ReplaceText == nil || *dec2.ReplaceText != "Paris" {
		t.Fatalf("expected replaced text 'Paris', got %v", dec2.ReplaceText)
	}

	// 4. Expiration test
	// Modify timestamp to simulate expiry (older than 5 minutes)
	entry.timestamp = time.Now().Add(-6 * time.Minute)
	lc.cache["What is the capital of France?"] = entry

	req3 := &mw.Event{
		Name:     mw.EventBeforeLLMRequest,
		UserText: "What is the capital of France?",
	}
	dec3, err := lc.OnEvent(ctx, req3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec3.Cancel {
		t.Fatalf("expected miss after expiry, got cancel")
	}

	// Verify entry was removed
	if _, ok := lc.cache["What is the capital of France?"]; ok {
		t.Fatalf("expected cache entry to be removed after expiry")
	}
}
