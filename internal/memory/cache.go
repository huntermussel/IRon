package memory

import (
	"sync"
	"time"
)

// CacheEntry holds a single cached LLM response keyed by token fingerprint.
type CacheEntry struct {
	tokens map[string]struct{} // normalized query tokens
	resp   string
	at     time.Time
	ttl    time.Duration // 0 = never expires
}

func (e *CacheEntry) expired() bool {
	return e.ttl > 0 && time.Since(e.at) > e.ttl
}

// ResponseCache is a semantic cache for LLM responses.
// Similarity is measured via Jaccard coefficient on query token sets.
// A cache hit is returned when Jaccard(query, entry) >= threshold.
//
// Token savings: repeated or near-identical queries skip the LLM entirely.
// Typical threshold: 0.82 (very high overlap required to avoid false hits).
type ResponseCache struct {
	mu         sync.RWMutex
	entries    []CacheEntry
	threshold  float64
	maxEntries int
	defaultTTL time.Duration
}

// NewResponseCache creates a cache with the given Jaccard threshold (0–1),
// max capacity, and default TTL (0 = permanent).
func NewResponseCache(threshold float64, maxEntries int, ttl time.Duration) *ResponseCache {
	if threshold <= 0 || threshold > 1 {
		threshold = 0.82
	}
	if maxEntries <= 0 {
		maxEntries = 256
	}
	return &ResponseCache{
		threshold:  threshold,
		maxEntries: maxEntries,
		defaultTTL: ttl,
	}
}

// Set stores a query → response pair.
// Queries shorter than 3 tokens are not cached (too ambiguous).
func (c *ResponseCache) Set(query, response string) {
	toks := tokenSet(query)
	if len(toks) < 3 || response == "" {
		return
	}
	entry := CacheEntry{
		tokens: toks,
		resp:   response,
		at:     time.Now(),
		ttl:    c.defaultTTL,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.evictLocked()
	if len(c.entries) >= c.maxEntries {
		c.entries = c.entries[1:] // evict oldest
	}
	c.entries = append(c.entries, entry)
}

// Get looks up a semantically similar cached response.
// Returns ("", false) on miss or when the best match is below threshold.
func (c *ResponseCache) Get(query string) (string, bool) {
	qToks := tokenSet(query)
	if len(qToks) < 3 {
		return "", false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	var bestResp string
	bestScore := 0.0

	for i := range c.entries {
		e := &c.entries[i]
		if e.expired() {
			continue
		}
		if sim := jaccard(qToks, e.tokens); sim > bestScore {
			bestScore = sim
			bestResp = e.resp
		}
	}

	if bestScore >= c.threshold {
		return bestResp, true
	}
	return "", false
}

// Len returns the number of live (non-expired) entries.
func (c *ResponseCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	count := 0
	for i := range c.entries {
		if !c.entries[i].expired() {
			count++
		}
	}
	return count
}

func (c *ResponseCache) evictLocked() {
	alive := c.entries[:0]
	for _, e := range c.entries {
		if !e.expired() {
			alive = append(alive, e)
		}
	}
	c.entries = alive
}

// jaccard computes the Jaccard similarity coefficient: |A∩B| / |A∪B|.
func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	for k := range a {
		if _, ok := b[k]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
