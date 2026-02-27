package signal

import (
	"context"
	"log"
	"os"

	"iron/internal/communicators"
	"iron/internal/gateway"
)

func init() {
	communicators.Register(&Adapter{})
}

// Adapter runs IRon as a Signal Bot.
type Adapter struct {
}

func (a *Adapter) ID() string {
	return "signal"
}

// Start begins listening for Signal messages.
func (a *Adapter) Start(ctx context.Context, gw *gateway.Gateway) error {
	endpoint := os.Getenv("SIGNAL_ENDPOINT")
	if endpoint == "" {
		log.Println("[Signal] Disabled: SIGNAL_ENDPOINT environment variable not set")
		return nil
	}

	log.Printf("[Signal] 🤖 Starting Bot on endpoint %s... (Placeholder/Not fully implemented yet)", endpoint)

	// Placeholder loop to prevent immediate exit
	<-ctx.Done()
	log.Println("[Signal] Shutting down...")

	return nil
}
