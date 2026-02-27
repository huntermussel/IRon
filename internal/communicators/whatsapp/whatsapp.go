package whatsapp

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

// Adapter runs IRon as a WhatsApp Bot.
type Adapter struct {
}

func (a *Adapter) ID() string {
	return "whatsapp"
}

// Start begins listening for WhatsApp messages.
func (a *Adapter) Start(ctx context.Context, gw *gateway.Gateway) error {
	token := os.Getenv("WHATSAPP_TOKEN")
	if token == "" {
		log.Println("[WhatsApp] Disabled: WHATSAPP_TOKEN environment variable not set")
		return nil
	}

	log.Printf("[WhatsApp] 🤖 Starting Bot... (Placeholder/Not fully implemented yet)")

	// Placeholder loop to prevent immediate exit
	<-ctx.Done()
	log.Println("[WhatsApp] Shutting down...")

	return nil
}
