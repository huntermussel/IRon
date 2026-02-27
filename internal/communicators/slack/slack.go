package slack

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

// Adapter runs IRon as a Slack Bot.
type Adapter struct {
}

func (a *Adapter) ID() string {
	return "slack"
}

// Start begins listening for Slack messages.
func (a *Adapter) Start(ctx context.Context, gw *gateway.Gateway) error {
	token := os.Getenv("SLACK_BOT_TOKEN")
	if token == "" {
		log.Println("[Slack] Disabled: SLACK_BOT_TOKEN environment variable not set")
		return nil
	}

	log.Printf("[Slack] 🤖 Starting Bot... (Placeholder/Not fully implemented yet)")

	// Placeholder loop to prevent immediate exit
	<-ctx.Done()
	log.Println("[Slack] Shutting down...")

	return nil
}
