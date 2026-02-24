package localcache

import (
	"context"
	"sync"
	"time"

	mw "iron/internal/middleware"
)

func init() {
	mw.Register(&LocalCache{
		cache: make(map[string]cacheEntry),
	})
}

type cacheEntry struct {
	response  string
	timestamp time.Time
}

// LocalCache skips the LLM if the exact prompt was answered recently.
type LocalCache struct {
	mu    sync.RWMutex
	cache map[string]cacheEntry
}

func (l *LocalCache) ID() string {
	return "local-cache"
}

func (l *LocalCache) Priority() int {
	// Run after greeting (110) and IntentCompressor (90) but before LLM call.
	return 80
}

func (l *LocalCache) OnEvent(ctx context.Context, e *mw.Event) (mw.Decision, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Clean up old entries periodically or on write (lazy cleanup for simplicity here)
	// For this demo, we'll just check expiration on read.

	switch e.Name {
	case mw.EventBeforeLLMRequest:
		if entry, ok := l.cache[e.UserText]; ok {
			// Check if entry is "recent" (e.g., within 5 minutes)
			if time.Since(entry.timestamp) < 5*time.Minute {
				reply := entry.response
				return mw.Decision{
					Cancel:      true,
					ReplaceText: &reply,
					Reason:      "served from local cache",
				}, nil
			} else {
				// Expired
				delete(l.cache, e.UserText)
			}
		}
	case mw.EventAfterLLMResponse:
		// Cache the response
		if e.UserText != "" && e.LLMText != "" {
			l.cache[e.UserText] = cacheEntry{
				response:  e.LLMText,
				timestamp: time.Now(),
			}
		}
	}

	return mw.Decision{}, nil
}
