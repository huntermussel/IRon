package telegram

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"iron/internal/chat"
	"iron/internal/communicators"
	"iron/internal/gateway"

	tele "gopkg.in/telebot.v3"
)

var tLog *log.Logger

func init() {
	communicators.Register(&Adapter{})
	os.MkdirAll("bin", 0755)
	if f, err := os.OpenFile("bin/telegram.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
		tLog = log.New(f, "", log.LstdFlags)
	} else {
		tLog = log.New(os.Stderr, "", log.LstdFlags)
	}
}

type userSession struct {
	service *chat.Service
	cleanup func()
	lastUse time.Time
	mu      sync.Mutex
}

// Adapter runs IRon as a Telegram Bot.
type Adapter struct {
	bot        *tele.Bot
	gw         *gateway.Gateway
	sessions   map[int64]*userSession
	sessionsMu sync.RWMutex
	logChatID  int64
}

func (a *Adapter) ID() string {
	return "telegram"
}

// Start begins listening for Telegram messages.
func (a *Adapter) Start(ctx context.Context, gw *gateway.Gateway) error {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		tLog.Println("[Telegram] Disabled: TELEGRAM_BOT_TOKEN environment variable not set")
		return nil
	}

	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		return fmt.Errorf("failed to create telegram bot: %w", err)
	}

	a.bot = b
	a.gw = gw
	a.sessions = make(map[int64]*userSession)

	if logChatStr := os.Getenv("TELEGRAM_LOG_CHAT_ID"); logChatStr != "" {
		if id, err := strconv.ParseInt(logChatStr, 10, 64); err == nil {
			a.logChatID = id
		}
	}

	a.setupHandlers()

	a.log("[Telegram] 🤖 Starting Bot... (@%s)", a.bot.Me.Username)

	// Clean up old sessions periodically
	go a.cleanupLoop(ctx)

	go func() {
		<-ctx.Done()
		a.log("[Telegram] Shutting down...")
		a.bot.Stop()
		a.cleanupAllSessions()
	}()

	a.bot.Start()
	return nil
}

func (a *Adapter) setupHandlers() {
	a.bot.Handle("/start", func(c tele.Context) error {
		return c.Send("👋 Welcome to IRon! I am your personal AI assistant. How can I help you today?")
	})

	a.bot.Handle("/clear", func(c tele.Context) error {
		chatID := c.Chat().ID
		a.sessionsMu.Lock()
		if session, exists := a.sessions[chatID]; exists {
			session.mu.Lock()
			session.service.Clear()
			session.mu.Unlock()
		}
		a.sessionsMu.Unlock()
		return c.Send("🧹 Conversation context cleared.")
	})

	a.bot.Handle(tele.OnText, a.handleMessage)
}

func (a *Adapter) handleMessage(c tele.Context) error {
	chatID := c.Chat().ID
	text := c.Text()

	// Provide typing indicator to the user
	_ = c.Notify(tele.Typing)

	session, err := a.getSession(context.Background(), chatID)
	if err != nil {
		a.log("[Telegram] Error getting session for %d: %v", chatID, err)
		return c.Send("⚠️ Error initializing assistant. Please try again later.")
	}

	session.mu.Lock()
	defer session.mu.Unlock()
	session.lastUse = time.Now()

	// Give a generous timeout for complex operations via Telegram
	turnCtx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	mwCtx := map[string]any{
		"telegram_chat_id": chatID,
	}

	reply, err := session.service.SendWithContext(turnCtx, text, mwCtx)
	if err != nil {
		a.log("[Telegram] Error processing message for %d: %v", chatID, err)
		return c.Send(fmt.Sprintf("⚠️ An error occurred: %v", err))
	}

	if reply == "" {
		return c.Send("🤷‍♂️ I don't have a response for that.")
	}

	// Telegram messages have a 4096 character limit
	return sendLongMessage(a.bot, c.Chat(), reply)
}

func (a *Adapter) getSession(ctx context.Context, chatID int64) (*userSession, error) {
	a.sessionsMu.RLock()
	session, exists := a.sessions[chatID]
	a.sessionsMu.RUnlock()

	if exists {
		return session, nil
	}

	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()

	// Check again in case it was created while waiting for the lock
	if session, exists := a.sessions[chatID]; exists {
		return session, nil
	}

	a.log("[Telegram] Initializing new IRon session for chat %d...", chatID)

	statusCb := func(msg string) {
		a.log("%s", msg)
	}
	streamCb := func(msg string) {
		// no-op to prevent stdout spam
	}

	service, _, _, _, cleanup, err := a.gw.InitService(ctx, chat.WithStatusCallback(statusCb), chat.WithStreamCallback(streamCb))
	if err != nil {
		return nil, err
	}

	session = &userSession{
		service: service,
		cleanup: cleanup,
		lastUse: time.Now(),
	}
	a.sessions[chatID] = session

	return session, nil
}

func (a *Adapter) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.sessionsMu.Lock()
			for id, session := range a.sessions {
				// Expire sessions inactive for more than 2 hours
				if time.Since(session.lastUse) > 2*time.Hour {
					a.log("[Telegram] Cleaning up inactive session for chat %d", id)
					session.cleanup()
					delete(a.sessions, id)
				}
			}
			a.sessionsMu.Unlock()
		}
	}
}

func (a *Adapter) cleanupAllSessions() {
	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()
	for _, session := range a.sessions {
		session.cleanup()
	}
	a.sessions = make(map[int64]*userSession)
}

// sendLongMessage splits and sends text if it exceeds Telegram's 4096 char limit.
func sendLongMessage(bot *tele.Bot, to tele.Recipient, text string, opts ...interface{}) error {
	const maxLen = 4000 // Leave a little buffer
	var err error

	for len(text) > 0 {
		if len(text) > maxLen {
			chunk := text[:maxLen]
			_, err = bot.Send(to, chunk, opts...)
			text = text[maxLen:]
		} else {
			_, err = bot.Send(to, text, opts...)
			text = ""
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *Adapter) log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	tLog.Println(msg)
	if a.logChatID != 0 && a.bot != nil {
		_ = sendLongMessage(a.bot, &tele.Chat{ID: a.logChatID}, "🪵 "+msg)
	}
}
