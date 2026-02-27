package middleware

import (
	"io"
	"os"
	"strings"
)

// registry holds globally-registered middleware plugins.
var registry []Middleware

// Register should be called by middleware packages (typically in init) to
// register themselves with the core chain builder.
func Register(m Middleware) {
	registry = append(registry, m)
}

// Registered returns a shallow copy of all registered middleware.
func Registered() []Middleware {
	out := make([]Middleware, len(registry))
	copy(out, registry)
	return out
}

// NewChainFromRegistry builds a chain from all registered middleware.
// If a debug writer is provided, it is attached for JSONL debug logs.
func NewChainFromRegistry(debugWriter io.Writer) *Chain {
	mws := Registered()

	// Filter out disabled middlewares via IRON_DISABLED_MIDDLEWARES env var.
	if disabled := os.Getenv("IRON_DISABLED_MIDDLEWARES"); disabled != "" {
		disabledSet := make(map[string]struct{})
		for _, id := range strings.Split(disabled, ",") {
			disabledSet[strings.TrimSpace(id)] = struct{}{}
		}

		filtered := make([]Middleware, 0, len(mws))
		for _, mw := range mws {
			if _, ok := disabledSet[mw.ID()]; !ok {
				filtered = append(filtered, mw)
			}
		}
		mws = filtered
	}

	if len(mws) == 0 {
		return nil
	}
	c := NewChain(mws...)
	if debugWriter != nil {
		c.SetDebugWriter(debugWriter)
	}
	return c
}
