package middleware

import (
	"context"
	"io"
	"sort"
	"sync"
)

// Chain executes middlewares in descending Priority() order.
// If priorities are equal, registration order is preserved.
type Chain struct {
	mu  sync.RWMutex
	mws []Middleware

	debugMu sync.Mutex
	debugW  io.Writer
}

type DecisionResult struct {
	MiddlewareID string
	Priority     int
	Decision     Decision
}

/**
 *
 * mws: a slice of Middleware instances to be used in the chain.
 * mw: a Middleware instance to be used in the chain.
 */
func NewChain(mws ...Middleware) *Chain {
	c := &Chain{}
	for _, mw := range mws {
		c.Use(mw)
	}
	return c
}

// SetDebugWriter enables JSONL debug logging for dispatch decisions.
// If w is nil, logging is disabled.
func (c *Chain) SetDebugWriter(w io.Writer) {
	c.debugMu.Lock()
	defer c.debugMu.Unlock()
	c.debugW = w
}

func (c *Chain) Use(mw Middleware) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mws = append(c.mws, mw)
	c.sortLocked()
}

func (c *Chain) List() []Middleware {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Middleware, len(c.mws))
	copy(out, c.mws)
	return out
}

// Dispatch runs all middlewares for the given event, stopping early if a middleware returns Decision.Cancel.
func (c *Chain) Dispatch(ctx context.Context, e *Event) ([]DecisionResult, error) {
	c.mu.RLock()
	mws := make([]Middleware, len(c.mws))
	copy(mws, c.mws)
	c.mu.RUnlock()

	results := make([]DecisionResult, 0, len(mws))
	for _, mw := range mws {
		beforeText := eventText(e)
		if cmw, ok := mw.(ConditionalMiddleware); ok && !cmw.ShouldLoad(ctx, e) {
			c.debugLog(e, mw.ID(), mw.Priority(), true, beforeText, beforeText, Decision{Reason: "skipped (ShouldLoad=false)"})
			results = append(results, DecisionResult{
				MiddlewareID: mw.ID(),
				Priority:     mw.Priority(),
				Decision: Decision{
					Reason: "skipped (ShouldLoad=false)",
				},
			})
			continue
		}

		dec, err := mw.OnEvent(ctx, e)
		if err != nil {
			c.debugLog(e, mw.ID(), mw.Priority(), false, beforeText, eventText(e), Decision{Reason: err.Error(), Cancel: true})
			return nil, err
		}

		applyDecisionToEvent(e, dec)
		afterText := eventText(e)
		c.debugLog(e, mw.ID(), mw.Priority(), false, beforeText, afterText, dec)

		// Keep a record even if the decision is "no-op" (all fields zero),
		// since callers may want visibility/logging per middleware.
		results = append(results, DecisionResult{
			MiddlewareID: mw.ID(),
			Priority:     mw.Priority(),
			Decision:     dec,
		})
		if dec.Cancel {
			break
		}
	}
	return results, nil
}

func (c *Chain) sortLocked() {
	sort.SliceStable(c.mws, func(i, j int) bool {
		return c.mws[i].Priority() > c.mws[j].Priority()
	})
}
