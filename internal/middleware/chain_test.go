package middleware

import (
	"context"
	"testing"
)

type testMW struct {
	id       string
	priority int
	cancel   bool
	seen     *[]string
}

func (m testMW) ID() string    { return m.id }
func (m testMW) Priority() int { return m.priority }
func (m testMW) OnEvent(_ context.Context, _ *Event) (Decision, error) {
	*m.seen = append(*m.seen, m.id)
	return Decision{Cancel: m.cancel}, nil
}

type conditionalTestMW struct {
	testMW
	enabled bool
}

func (m conditionalTestMW) ShouldLoad(_ context.Context, _ *Event) bool { return m.enabled }

func TestChainPriorityAndCancel(t *testing.T) {
	seen := []string{}
	c := NewChain(
		testMW{id: "low", priority: 1, seen: &seen},
		testMW{id: "high", priority: 10, cancel: true, seen: &seen},
		testMW{id: "mid", priority: 5, seen: &seen},
	)

	_, err := c.Dispatch(context.Background(), &Event{Name: EventBeforeLLMRequest})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(seen) != 1 || seen[0] != "high" {
		t.Fatalf("expected only high to run (cancel), got %v", seen)
	}
}

func TestChainConditionalMiddlewareSkip(t *testing.T) {
	seen := []string{}
	c := NewChain(
		conditionalTestMW{testMW: testMW{id: "off", priority: 10, seen: &seen}, enabled: false},
		conditionalTestMW{testMW: testMW{id: "on", priority: 5, seen: &seen}, enabled: true},
	)

	results, err := c.Dispatch(context.Background(), &Event{Name: EventBeforeLLMRequest})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := join(seen); got != "on" {
		t.Fatalf("expected only enabled middleware to run, got %s", got)
	}
	if len(results) != 2 {
		t.Fatalf("expected results for both middlewares, got %d", len(results))
	}
	if results[0].MiddlewareID != "off" || results[0].Decision.Reason == "" {
		t.Fatalf("expected first result to be skipped middleware with a reason, got %+v", results[0])
	}
}

func TestChainStableOrderOnEqualPriority(t *testing.T) {
	seen := []string{}
	c := NewChain(
		testMW{id: "a", priority: 5, seen: &seen},
		testMW{id: "b", priority: 5, seen: &seen},
		testMW{id: "c", priority: 5, seen: &seen},
	)

	_, err := c.Dispatch(context.Background(), &Event{Name: EventBeforeLLMRequest})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := join(seen); got != "a,b,c" {
		t.Fatalf("expected stable registration order, got %s", got)
	}
}

func join(in []string) string {
	if len(in) == 0 {
		return ""
	}
	out := in[0]
	for i := 1; i < len(in); i++ {
		out += "," + in[i]
	}
	return out
}
