package telegram

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"iron/internal/chat"
	"iron/internal/gateway"

	"github.com/joho/godotenv"
	tele "gopkg.in/telebot.v3"
)

type userSession struct {
	service *chat.Service
	cleanup func()
	lastUse time.Time
	mu      sync.Mutex
}

// BotAdapter runs IRon as a Telegram Bot.
type BotAdapter struct {
	bot        *tele.Bot
	gw         *gateway.Gateway
	sessions   map[int64]*userSession
	sessionsMu sync.RWMutex
}

// NewBot creates a new Telegram Bot adapter.
func NewBot(configPath string) (*BotAdapter, error) {
	_ = godotenv.Load()

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	adapter := &BotAdapter{
		bot:      b,
		gw:       gateway.New(configPath),
		sessions: make(map[int64]*userSession),
	}

	adapter.setupHandlers()
	return adapter, nil
}

// Start begins listening for Telegram messages.
func (b *BotAdapter) Start(ctx context.Context) error {
	log.Printf("🤖 Starting IRon Telegram Bot... (@%s)", b.bot.Me.Username)

	// Clean up old sessions periodically
	go b.cleanupLoop(ctx)

	go func() {
		<-ctx.Done()
		log.Println("Shutting down Telegram bot...")
		b.bot.Stop()
		b.cleanupAllSessions()
	}()

	b.bot.Start()
	return nil
}

func (b *BotAdapter) setupHandlers() {
	b.bot.Handle("/start", func(c tele.Context) error {
		return c.Send("👋 Welcome to IRon! I am your personal AI assistant. How can I help you today?")
	})

	b.bot.Handle("/clear", func(c tele.Context) error {
		chatID := c.Chat().ID
		b.sessionsMu.Lock()
		if session, exists := b.sessions[chatID]; exists {
			session.mu.Lock()
			session.service.Clear()
			session.mu.Unlock()
		}
		b.sessionsMu.Unlock()
		return c.Send("🧹 Conversation context cleared.")
	})

	b.bot.Handle(tele.OnText, b.handleMessage)
}

func (b *BotAdapter) handleMessage(c tele.Context) error {
	chatID := c.Chat().ID
	text := c.Text()

	// Provide typing indicator to the user
	_ = c.Notify(tele.Typing)

	session, err := b.getSession(context.Background(), chatID)
	if err != nil {
		log.Printf("Error getting session for %d: %v", chatID, err)
		return c.Send("⚠️ Error initializing assistant. Please try again later.")
	}

	session.mu.Lock()
	defer session.mu.Unlock()
	session.lastUse = time.Now()

	// Give a generous timeout for complex operations via Telegram
	turnCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	reply, err := session.service.Send(turnCtx, text)
	if err != nil {
		log.Printf("Error processing message for %d: %v", chatID, err)
		return c.Send(fmt.Sprintf("⚠️ An error occurred: %v", err))
	}

	if reply == "" {
		return c.Send("🤷‍♂️ I don't have a response for that.")
	}

	// Telegram messages have a 4096 character limit
	return sendLongMessage(c, reply)
}

func (b *BotAdapter) getSession(ctx context.Context, chatID int64) (*userSession, error) {
	b.sessionsMu.RLock()
	session, exists := b.sessions[chatID]
	b.sessionsMu.RUnlock()

	if exists {
		return session, nil
	}

	b.sessionsMu.Lock()
	defer b.sessionsMu.Unlock()

	// Check again in case it was created while waiting for the lock
	if session, exists := b.sessions[chatID]; exists {
		return session, nil
	}

	log.Printf("Initializing new IRon session for chat %d...", chatID)
	service, _, _, _, cleanup, err := b.gw.InitService(ctx)
	if err != nil {
		return nil, err
	}

	session = &userSession{
		service: service,
		cleanup: cleanup,
		lastUse: time.Now(),
	}
	b.sessions[chatID] = session

	return session, nil
}

func (b *BotAdapter) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.sessionsMu.Lock()
			for id, session := range b.sessions {
				// Expire sessions inactive for more than 2 hours
				if time.Since(session.lastUse) > 2*time.Hour {
					log.Printf("Cleaning up inactive session for chat %d", id)
					session.cleanup()
					delete(b.sessions, id)
				}
			}
			b.sessionsMu.Unlock()
		}
	}
}

func (b *BotAdapter) cleanupAllSessions() {
	b.sessionsMu.Lock()
	defer b.sessionsMu.Unlock()
	for _, session := range b.sessions {
		session.cleanup()
	}
	b.sessions = make(map[int64]*userSession)
}

// sendLongMessage splits and sends text if it exceeds Telegram's 4096 char limit.
func sendLongMessage(c tele.Context, text string) error {
	const maxLen = 4000 // Leave a little buffer
	var err error

	for len(text) > 0 {
		if len(text) > maxLen {
			chunk := text[:maxLen]
			err = c.Send(chunk)
			text = text[maxLen:]
		} else {
			err = c.Send(text)
			text = ""
		}
		if err != nil {
			return err
		}
	}
	return nil
}
