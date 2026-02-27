package communicators

import (
	"context"
	"fmt"
	"iron/internal/gateway"
	"sync"
)

// Communicator defines the interface for an external chat adapter (e.g. CLI, Telegram, Slack).
type Communicator interface {
	// ID returns the unique name of the communicator (e.g., "telegram", "cli")
	ID() string

	// Start begins listening for events and routing them to IRon's core.
	// It should block until the context is canceled or an error occurs.
	Start(ctx context.Context, gw *gateway.Gateway) error
}

var (
	registry   = make(map[string]Communicator)
	registryMu sync.RWMutex
)

// Register adds a new Communicator to the global registry.
// This is typically called in an init() function of the specific adapter package.
func Register(c Communicator) {
	registryMu.Lock()
	defer registryMu.Unlock()

	if c == nil {
		panic("communicator: Register communicator is nil")
	}
	if _, dup := registry[c.ID()]; dup {
		panic("communicator: Register called twice for communicator " + c.ID())
	}

	registry[c.ID()] = c
}

// Get returns a registered communicator by ID.
func Get(id string) (Communicator, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	c, ok := registry[id]
	if !ok {
		return nil, fmt.Errorf("communicator '%s' not found", id)
	}
	return c, nil
}

// All returns a list of all registered communicators.
func All() []Communicator {
	registryMu.RLock()
	defer registryMu.RUnlock()

	var list []Communicator
	for _, c := range registry {
		list = append(list, c)
	}
	return list
}
