package tokenbudget

import (
	"context"
	mw "iron/internal/middleware"
)

func init() {
	// Auto-register middleware so it is picked up via middlewares/autoload.
	mw.Register(BudgetLimiter{})
}

// BudgetLimiter sets a maximum LLM token budget for a request if provided in
// Event.Context["token_budget"] (int). It prefers the smaller of existing
// MaxTokens and the provided budget.
type BudgetLimiter struct{}

func (BudgetLimiter) ID() string    { return "token_budget" }
func (BudgetLimiter) Priority() int { return 90 }

// ShouldLoad always returns true; the middleware will no-op when no budget is
// present in Event.Context["token_budget"].
func (BudgetLimiter) ShouldLoad(_ context.Context, _ *mw.Event) bool { return true }

func (BudgetLimiter) OnEvent(_ context.Context, e *mw.Event) (mw.Decision, error) {
	if e == nil || e.Name != mw.EventBeforeLLMRequest {
		return mw.Decision{}, nil
	}
	budget, ok := e.Context["token_budget"].(int)
	if !ok || budget <= 0 {
		return mw.Decision{}, nil
	}

	// Copy params so downstream can mutate safely.
	params := &mw.LLMParams{}
	if e.Params != nil {
		*params = *e.Params
	}

	if params.MaxTokens == 0 || params.MaxTokens > budget {
		params.MaxTokens = budget
		return mw.Decision{
			OverrideParams: params,
			Reason:         "token_budget: capped MaxTokens",
		}, nil
	}

	return mw.Decision{}, nil
}
