package middleware

import "io"

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
	if len(mws) == 0 {
		return nil
	}
	c := NewChain(mws...)
	if debugWriter != nil {
		c.SetDebugWriter(debugWriter)
	}
	return c
}
