package websearch

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"iron/internal/nlu"
	"iron/internal/proxy"
)

type SearchResult struct {
	Title   string
	URL     string
	Snippet string
}

type WebSearchMiddleware struct {
	nlu        *nlu.Router
	maxResults int
	timeout    time.Duration
}

func New(nlu *nlu.Router, maxResults, timeoutSec int) *WebSearchMiddleware {
	return &WebSearchMiddleware{nlu: nlu, maxResults: maxResults, timeout: time.Duration(timeoutSec) * time.Second}
}

func (m *WebSearchMiddleware) Name() string { return "websearch" }

func (m *WebSearchMiddleware) Handle(req *proxy.ChatRequest, next func(*proxy.ChatRequest) (*proxy.ChatResponse, error)) (*proxy.ChatResponse, error) {
	ctx := context.Background()
	query := lastUserMessage(req)
	if query == "" {
		return next(req)
	}
	intent, _ := m.nlu.Classify(ctx, query)
	if intent == nil || intent.Intent != nlu.QueryIntentWebSearch {
		return next(req)
	}
	results, err := m.search(query)
	if err != nil {
		log.Printf("[websearch] search failed: %v", err)
		return next(req)
	}
	formatted := formatResults(results)
	req.Messages = append(req.Messages, proxy.ChatMessage{Role: "system", Content: formatted})
	return next(req)
}

func (m *WebSearchMiddleware) search(query string) ([]SearchResult, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", strings.ReplaceAll(query, " ", "+"))
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := &http.Client{Timeout: m.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}
	var results []SearchResult
	doc.Find(".result").EachWithBreak(func(i int, s *goquery.Selection) bool {
		if i >= m.maxResults {
			return false
		}
		title := strings.TrimSpace(s.Find(".result__title a").Text())
		url, _ := s.Find(".result__title a").Attr("href")
		snippet := strings.TrimSpace(s.Find(".result__snippet").Text())
		snippet = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(snippet, "")
		results = append(results, SearchResult{Title: title, URL: url, Snippet: snippet})
		return true
	})
	return results, nil
}

func formatResults(results []SearchResult) string {
	b := "[WEB SEARCH RESULTS]\n"
	for i, r := range results {
		b += fmt.Sprintf("%d. %q - %s\nSnippet: %s\n", i+1, r.Title, r.URL, r.Snippet)
	}
	b += "[/WEB SEARCH RESULTS]\n"
	return b
}

func lastUserMessage(req *proxy.ChatRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return req.Messages[i].Content
		}
	}
	return ""
}
