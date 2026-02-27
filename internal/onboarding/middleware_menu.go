package onboarding

import (
	"bufio"
	"fmt"
	"iron/internal/middleware"
	"os"
	"sort"
	"strconv"
	"strings"
)

// MiddlewareSetting holds the user's choice for a specific middleware
type MiddlewareSetting struct {
	ID      string            `json:"id"`
	Enabled bool              `json:"enabled"`
	EnvVars map[string]string `json:"env_vars,omitempty"`
}

// MiddlewareMenu provides a CLI interface to select and configure middlewares
type MiddlewareMenu struct {
	scanner *bufio.Scanner
}

func NewMiddlewareMenu(scanner *bufio.Scanner) *MiddlewareMenu {
	if scanner == nil {
		scanner = bufio.NewScanner(os.Stdin)
	}
	return &MiddlewareMenu{
		scanner: scanner,
	}
}

// Run displays the menu and returns the gathered settings
func (m *MiddlewareMenu) Run() ([]MiddlewareSetting, error) {
	fmt.Println("\n🔧 Middleware & Features Configuration")
	fmt.Println("Enable or disable specific behaviors and set their parameters.")

	// Get all registered middlewares
	registered := middleware.Registered()
	sort.Slice(registered, func(i, j int) bool {
		return registered[i].ID() < registered[j].ID()
	})

	settings := make([]MiddlewareSetting, len(registered))
	for i, mw := range registered {
		settings[i] = MiddlewareSetting{
			ID:      mw.ID(),
			Enabled: true, // Default to enabled
			EnvVars: make(map[string]string),
		}
	}

	for {
		fmt.Println("\nAvailable Middlewares:")
		fmt.Println(strings.Repeat("-", 30))
		for i, s := range settings {
			status := "✅ [ON] "
			if !s.Enabled {
				status = "❌ [OFF]"
			}
			fmt.Printf("%2d) %s %-20s\n", i+1, status, s.ID)
		}
		fmt.Println(" 0) Finish & Save")

		fmt.Print("\nSelect a number to toggle/configure (or 0 to exit): ")
		m.scanner.Scan()
		input := strings.TrimSpace(m.scanner.Text())

		if input == "0" || input == "" {
			break
		}

		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(settings) {
			fmt.Println("⚠️  Invalid selection. Please try again.")
			continue
		}

		m.configureMiddleware(&settings[idx-1])
	}

	return settings, nil
}

func (m *MiddlewareMenu) configureMiddleware(s *MiddlewareSetting) {
	for {
		fmt.Printf("\n--- Configuring: %s ---\n", s.ID)
		statusStr := "ENABLED"
		if !s.Enabled {
			statusStr = "DISABLED"
		}
		fmt.Printf("Status: %s\n\n", statusStr)
		fmt.Println("1) Toggle Status")

		// Dynamic options based on Middleware ID
		options := getKnownEnvVars(s.ID)
		for i, opt := range options {
			currentVal := s.EnvVars[opt.Key]
			if currentVal == "" {
				currentVal = os.Getenv(opt.Key)
			}
			displayVal := currentVal
			if displayVal == "" {
				displayVal = "(not set)"
			}
			fmt.Printf("%d) %s: %s\n", i+2, opt.Label, displayVal)
		}

		fmt.Println("0) Back to Main List")
		fmt.Print("\nChoice: ")

		m.scanner.Scan()
		choice := strings.TrimSpace(m.scanner.Text())

		if choice == "0" || choice == "" {
			break
		}

		if choice == "1" {
			s.Enabled = !s.Enabled
			continue
		}

		optIdx, err := strconv.Atoi(choice)
		if err == nil && optIdx >= 2 && optIdx < len(options)+2 {
			opt := options[optIdx-2]
			fmt.Printf("Enter value for %s: ", opt.Label)
			m.scanner.Scan()
			val := strings.TrimSpace(m.scanner.Text())
			s.EnvVars[opt.Key] = val
		}
	}
}

type envOption struct {
	Label string
	Key   string
}

// getKnownEnvVars returns configuration keys for built-in middlewares
func getKnownEnvVars(id string) []envOption {
	switch id {
	case "weather":
		return []envOption{
			{Label: "Weather Provider API Key", Key: "WEATHER_API_KEY"},
			{Label: "Default Location", Key: "WEATHER_DEFAULT_LOCATION"},
		}
	case "pytools_mode", "pytools_exec":
		return []envOption{
			{Label: "Scripts Directory", Key: "IRON_SCRIPTS_DIR"},
			{Label: "Python Path", Key: "IRON_PYTHON_PATH"},
		}
	case "token_budget":
		return []envOption{
			{Label: "Max Tokens per Request", Key: "IRON_TOKEN_LIMIT"},
		}
	case "coding_tools_mode":
		return []envOption{
			{Label: "Restricted Root Path", Key: "IRON_CODING_ROOT"},
		}
	case "slack":
		return []envOption{
			{Label: "Slack Bot Token", Key: "SLACK_BOT_TOKEN"},
			{Label: "Slack App Token", Key: "SLACK_APP_TOKEN"},
		}
	default:
		return nil
	}
}

// UpdateEnvFile takes the middleware settings and appends them to the .env content
func (m *MiddlewareMenu) UpdateEnvFile(settings []MiddlewareSetting, envContent []string) []string {
	disabledList := []string{}
	for _, s := range settings {
		if !s.Enabled {
			disabledList = append(disabledList, s.ID)
		}
		for key, val := range s.EnvVars {
			if val != "" {
				envContent = append(envContent, fmt.Sprintf("%s=%s", key, val))
			}
		}
	}

	if len(disabledList) > 0 {
		envContent = append(envContent, fmt.Sprintf("IRON_DISABLED_MIDDLEWARES=%s", strings.Join(disabledList, ",")))
	}

	return envContent
}
