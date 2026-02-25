package browser

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// Config holds browser configuration.
type Config struct {
	Enabled     bool
	Headless    bool
	ChromePath  string
	UserDataDir string
}

// Controller manages a headless Chrome/Chromium instance.
type Controller struct {
	cfg         Config
	allocCtx    context.Context
	allocCancel context.CancelFunc
}

// New creates a browser controller.
func New(cfg Config) *Controller {
	return &Controller{cfg: cfg}
}

// Start launches Chrome/Chromium.
func (c *Controller) Start(ctx context.Context) error {
	if !c.cfg.Enabled {
		return nil
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", c.cfg.Headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.WindowSize(1280, 900),
	)
	if c.cfg.ChromePath != "" {
		opts = append(opts, chromedp.ExecPath(c.cfg.ChromePath))
	}
	if c.cfg.UserDataDir != "" {
		opts = append(opts, chromedp.UserDataDir(c.cfg.UserDataDir))
	}

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	c.allocCtx = allocCtx
	c.allocCancel = cancel

	// Trigger start by creating a context
	ctx, _ = chromedp.NewContext(c.allocCtx)
	if err := chromedp.Run(ctx); err != nil {
		return fmt.Errorf("failed to start browser: %w", err)
	}

	return nil
}

// Stop gracefully shuts down Chrome.
func (c *Controller) Stop() {
	if c.allocCancel != nil {
		c.allocCancel()
	}
}

// NavigateAndExtract goes to a URL and extracts the visible text.
func (c *Controller) NavigateAndExtract(url string) (string, error) {
	if c.allocCtx == nil {
		return "", fmt.Errorf("browser not started")
	}

	ctx, cancel := chromedp.NewContext(c.allocCtx)
	defer cancel()
	ctx, cancelTimeout := context.WithTimeout(ctx, 30*time.Second)
	defer cancelTimeout()

	var body string
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.Text("body", &body, chromedp.ByQuery),
	)
	if err != nil {
		return "", fmt.Errorf("navigate failed: %w", err)
	}
	return stripTags(body), nil
}

// Screenshot captures a PNG screenshot of a URL.
func (c *Controller) Screenshot(url string) ([]byte, error) {
	if c.allocCtx == nil {
		return nil, fmt.Errorf("browser not started")
	}

	ctx, cancel := chromedp.NewContext(c.allocCtx)
	defer cancel()
	ctx, cancelTimeout := context.WithTimeout(ctx, 30*time.Second)
	defer cancelTimeout()

	var buf []byte
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.FullScreenshot(&buf, 90),
	)
	return buf, err
}

func stripTags(text string) string {
	// Simple cleanup of whitespace
	lines := strings.Split(text, "\n")
	var clean []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			clean = append(clean, l)
		}
	}
	return strings.Join(clean, "\n")
}
