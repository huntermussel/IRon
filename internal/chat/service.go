package chat

import (
	"context"
	"errors"
	"iron/internal/middleware"
	"strings"
)

type Service struct {
	adapter Adapter
	history []Message
	mws     *middleware.Chain
}

type ServiceOption func(*Service)

func WithMiddlewareChain(chain *middleware.Chain) ServiceOption {
	return func(s *Service) {
		s.mws = chain
	}
}

func NewService(adapter Adapter, opts ...ServiceOption) *Service {
	s := &Service{
		adapter: adapter,
		history: make([]Message, 0, 16),
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

	if s.mws != nil {
		e := &middleware.Event{
			Name:     middleware.EventBeforeLLMRequest,
			UserText: input,
			Context:  mwCtx,
		}
		results, err := s.mws.Dispatch(ctx, e)
		if err != nil {
			return "", err
		}
		updated, canceled := applyTextDecisions(input, results)
		if canceled != nil && canceled.Cancel {
			if strings.TrimSpace(updated) != "" {
				return updated, nil
			}
			if strings.TrimSpace(canceled.Reason) == "" {
				return "", errors.New("request canceled by middleware")
			}
			return "", errors.New(canceled.Reason)
		}
		input = updated
	}

	s.history = append(s.history, Message{Role: RoleUser, Content: input})
	assistant, err := s.adapter.Reply(ctx, s.history)
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
			UserText: input,
			LLMText:  assistant,
			Context:  mwCtx,
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
	return assistant, nil
}

func (s *Service) Clear() {
	s.history = s.history[:0]
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
