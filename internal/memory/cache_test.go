package memory

import (
	"testing"
	"time"
)

func TestResponseCache_SetAndGet_Hit(t *testing.T) {
	c := NewResponseCache(0.7, 64, 0)
	c.Set("how do I build a go binary", "use go build -o bin/app ./cmd/app")
	resp, ok := c.Get("how do I build a go binary")
	if !ok {
		t.Fatal("expected cache hit on exact query")
	}
	if resp == "" {
		t.Fatal("expected non-empty cached response")
	}
}

func TestResponseCache_Get_NearMatch(t *testing.T) {
	c := NewResponseCache(0.5, 64, 0)
	q := "how do I run the tests for this go project"
	c.Set(q, "use go test ./...")
	// Slightly different phrasing — same core tokens, different function words
	resp, ok := c.Get("how do I run all tests for this go project please")
	if !ok {
		t.Fatal("expected cache hit on similar query (threshold=0.5)")
	}
	if resp == "" {
		t.Fatal("expected non-empty cached response")
	}
}

func TestResponseCache_Get_Miss(t *testing.T) {
	c := NewResponseCache(0.82, 64, 0)
	c.Set("how do I build the binary", "go build ./...")
	// Totally different query
	_, ok := c.Get("what is the weather in london")
	if ok {
		t.Fatal("expected cache miss for unrelated query")
	}
}

func TestResponseCache_TTL_Expiry(t *testing.T) {
	c := NewResponseCache(0.7, 64, 10*time.Millisecond)
	c.Set("test query with enough tokens", "cached response text here")

	// Should hit before expiry
	_, ok := c.Get("test query with enough tokens")
	if !ok {
		t.Fatal("expected hit before TTL expiry")
	}

	time.Sleep(20 * time.Millisecond)

	_, ok = c.Get("test query with enough tokens")
	if ok {
		t.Fatal("expected miss after TTL expiry")
	}
}

func TestResponseCache_ShortQuery_NotCached(t *testing.T) {
	c := NewResponseCache(0.7, 64, 0)
	c.Set("hi", "hello there")
	_, ok := c.Get("hi")
	if ok {
		t.Fatal("short queries (<3 tokens) should not be cached")
	}
}

func TestResponseCache_Len(t *testing.T) {
	c := NewResponseCache(0.7, 64, 0)
	if c.Len() != 0 {
		t.Fatal("expected empty cache")
	}
	c.Set("first query about building go projects", "answer one")
	c.Set("second query about testing go code here", "answer two")
	if c.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", c.Len())
	}
}

func TestJaccard(t *testing.T) {
	a := tokenSet("the quick brown fox")
	b := tokenSet("the quick brown fox")
	if jaccard(a, b) != 1.0 {
		t.Errorf("identical sets: expected 1.0, got %f", jaccard(a, b))
	}

	c := tokenSet("completely different words here")
	if jaccard(a, c) > 0.1 {
		t.Errorf("unrelated sets: expected ~0, got %f", jaccard(a, c))
	}
}
