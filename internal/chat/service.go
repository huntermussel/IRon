package chat

import (
	"context"
	"encoding/json"
	"errors"
	"iron/internal/memory"
	"iron/internal/middleware"
	"strings"
)

type Service struct {
	adapter Adapter
	history []Message
	mws     *middleware.Chain
	mem     *memory.Store
}

type ServiceOption func(*Service)

func WithMiddlewareChain(chain *middleware.Chain) ServiceOption {
	return func(s *Service) {
		s.mws = chain
	}
}

func WithMemoryStore(mem *memory.Store) ServiceOption {
	return func(s *Service) {
		s.mem = mem
	}
}

func NewService(adapter Adapter, opts ...ServiceOption) *Service {
	s := &Service{
		adapter: adapter,
		history: make([]Message, 0, 16),
		mem:     memory.NewStore(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) Send(ctx context.Context, input string) (string, error) {
	return s.SendWithContext(ctx, input, nil)
}

func (s *Service) SendWithContext(ctx context.Context, input string, mwCtx map[string]any) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("empty input")
	}

	// Trim history to reduce tokens (keep last 8 messages).
	if len(s.history) > 8 {
		s.history = append([]Message{}, s.history[len(s.history)-8:]...)
	}

	// Retrieve lightweight IR snippets to prepend as context.
	inputWithContext := input
	if s.mem != nil {
		if hits := s.mem.Query("default", input, 2); len(hits) > 0 {
			inputWithContext = "Context:\n- " + strings.Join(hits, "\n- ") + "\n\n" + input
		}
	}

	var llmParams *middleware.LLMParams
	if s.mws != nil {
		if mwCtx == nil {
			mwCtx = map[string]any{}
		}
		e := &middleware.Event{
			Name:     middleware.EventBeforeLLMRequest,
			UserText: inputWithContext,
			Context:  mwCtx,
			Params:   &middleware.LLMParams{},
		}
		results, err := s.mws.Dispatch(ctx, e)
		if err != nil {
			return "", err
		}
		updated, canceled := applyTextDecisions(inputWithContext, results)
		if canceled != nil && canceled.Cancel {
			if strings.TrimSpace(updated) != "" {
				return updated, nil
			}
			if strings.TrimSpace(canceled.Reason) == "" {
				return "", errors.New("request canceled by middleware")
			}
			return "", errors.New(canceled.Reason)
		}
		inputWithContext = updated
		mwCtx = e.Context
		llmParams = e.Params
	}

	s.history = append(s.history, Message{Role: RoleUser, Content: inputWithContext})
	var streamed strings.Builder
	assistant, toolCalls, err := s.adapter.ReplyStream(ctx, s.history, llmParams, func(chunk string) {
		streamed.WriteString(chunk)
	})
	if err != nil {
		s.history = s.history[:len(s.history)-1]
		return "", err
	}
	assistant = strings.TrimSpace(assistant)
	if assistant == "" {
		s.history = s.history[:len(s.history)-1]
		return "", errors.New("empty response from model")
	}

	if s.mws != nil {
		e := &middleware.Event{
			Name:     middleware.EventAfterLLMResponse,
			UserText: inputWithContext,
			LLMText:  assistant,
			Context:  mwCtx,
		}
		if len(toolCalls) > 0 {
			tc := make([]middleware.ToolCall, 0, len(toolCalls))
			for _, c := range toolCalls {
				tc = append(tc, middleware.ToolCall{Tool: c.Name, Args: parseToolArgs(c.Arguments)})
			}
			if e.Context == nil {
				e.Context = map[string]any{}
			}
			e.Context["tool_calls"] = tc
		}
		results, err := s.mws.Dispatch(ctx, e)
		if err != nil {
			s.history = s.history[:len(s.history)-1]
			return "", err
		}
		updated, canceled := applyTextDecisions(assistant, results)
		if canceled != nil && canceled.Cancel {
			if strings.TrimSpace(updated) != "" {
				assistant = updated
			} else {
				s.history = s.history[:len(s.history)-1]
				if strings.TrimSpace(canceled.Reason) == "" {
					return "", errors.New("response canceled by middleware")
				}
				return "", errors.New(canceled.Reason)
			}
		} else {
			assistant = updated
		}
	}

	s.history = append(s.history, Message{Role: RoleAssistant, Content: assistant})

	// Index compact traces for future retrieval.
	if s.mem != nil {
		s.mem.Index("default", inputWithContext)
		s.mem.Index("default", assistant)
	}
	return assistant, nil
}

func (s *Service) Clear() {
	s.history = s.history[:0]
}

func parseToolArgs(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return map[string]any{"raw": raw}
	}
	if m == nil {
		return map[string]any{}
	}
	return m
}

func applyTextDecisions(initial string, results []middleware.DecisionResult) (string, *middleware.Decision) {
	cur := strings.TrimSpace(initial)
	for _, r := range results {
		dec := r.Decision
		if dec.ReplaceText != nil {
			cur = strings.TrimSpace(*dec.ReplaceText)
		}
		if dec.Cancel {
			return cur, &dec
		}
	}
	return cur, nil
}
