package nlu

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// IntentResult represents the result of an NLU parse operation.
type IntentResult struct {
	Intent     string
	Confidence float64
	Slots      map[string]string
}

// Engine manages intent matching using a registry of utterances.
type Engine struct {
	mu       sync.RWMutex
	matchers []*intentMatcher
}

// intentMatcher holds the compiled logic for a specific intent's utterances.
type intentMatcher struct {
	intentName string
	regex      *regexp.Regexp
	slotNames  []string
}

var globalEngine *Engine
var once sync.Once

// GetEngine returns the singleton NLU engine instance.
func GetEngine() *Engine {
	once.Do(func() {
		globalEngine = &Engine{
			matchers: make([]*intentMatcher, 0),
		}
	})
	return globalEngine
}

// RegisterIntent adds an intent with a list of example utterances.
// Utterances can contain entities in the format {entity_name}.
// Example: RegisterIntent("set_alarm", "set alarm for {time}", "wake me up at {time}")
func (e *Engine) RegisterIntent(intent string, utterances ...string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, u := range utterances {
		matcher, err := compileUtterance(intent, u)
		if err == nil {
			e.matchers = append(e.matchers, matcher)
		} else {
			fmt.Printf("Error compiling utterance '%s': %v\n", u, err)
		}
	}
}

// Parse attempts to match the input string against registered intents.
// It returns the best matching intent, its confidence, and extracted slots.
func (e *Engine) Parse(input string) IntentResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	input = strings.TrimSpace(input)

	for _, m := range e.matchers {
		if matches := m.regex.FindStringSubmatch(input); matches != nil {
			slots := make(map[string]string)
			// The submatches array includes the full match at index 0.
			// Captured groups start at index 1.
			for i, name := range m.slotNames {
				if i+1 < len(matches) {
					slots[name] = strings.TrimSpace(matches[i+1])
				}
			}

			return IntentResult{
				Intent:     m.intentName,
				Confidence: 1.0,
				Slots:      slots,
			}
		}
	}

	return IntentResult{
		Intent:     "",
		Confidence: 0.0,
		Slots:      nil,
	}
}

// compileUtterance converts a natural language template into a regex matcher.
// "set alarm for {time}" -> `(?i)^set\s+alarm\s+for\s+(?P<time>.+)$`
func compileUtterance(intent, utterance string) (*intentMatcher, error) {
	// Normalize spaces in utterance first to single space
	utterance = strings.Join(strings.Fields(utterance), " ")

	var regexParts []string
	var slotNames []string

	// Split by '{' to find start of slots
	segments := strings.Split(utterance, "{")

	// First segment is static prefix (e.g. "set alarm for ")
	if len(segments) > 0 {
		prefix := segments[0]
		// Escape regex meta chars in static text
		escaped := regexp.QuoteMeta(prefix)
		// Replace explicit spaces with \s+ to be flexible
		escaped = strings.ReplaceAll(escaped, " ", `\s+`)
		regexParts = append(regexParts, escaped)
	}

	for i := 1; i < len(segments); i++ {
		// segment looks like "time} optional suffix"
		parts := strings.SplitN(segments[i], "}", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("unclosed brace in utterance: %s", utterance)
		}

		slotName := strings.TrimSpace(parts[0])
		suffix := parts[1]

		slotNames = append(slotNames, slotName)

		// Add regex capture group for the slot.
		// Using (.*?) for non-greedy match is safer for multiple slots.
		regexParts = append(regexParts, `(.*?)`)

		if suffix != "" {
			escaped := regexp.QuoteMeta(suffix)
			escaped = strings.ReplaceAll(escaped, " ", `\s+`)
			regexParts = append(regexParts, escaped)
		}
	}

	// Case insensitive (?i), match from start ^ to end $
	fullPattern := `(?i)^` + strings.Join(regexParts, "") + `$`

	re, err := regexp.Compile(fullPattern)
	if err != nil {
		return nil, err
	}

	return &intentMatcher{
		intentName: intent,
		regex:      re,
		slotNames:  slotNames,
	}, nil
}
