package chat

import (
	"context"
	"errors"
	"strings"
)

type Service struct {
	adapter Adapter
	history []Message
}

func NewService(adapter Adapter) *Service {
	return &Service{
		adapter: adapter,
		history: make([]Message, 0, 16),
	}
}

func (s *Service) Send(ctx context.Context, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("empty input")
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

	s.history = append(s.history, Message{Role: RoleAssistant, Content: assistant})
	return assistant, nil
}

func (s *Service) Clear() {
	s.history = s.history[:0]
}
