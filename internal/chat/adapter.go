package chat

import "context"

// Adapter abstracts chat completion providers.
type Adapter interface {
	Reply(ctx context.Context, history []Message) (string, error)
}
