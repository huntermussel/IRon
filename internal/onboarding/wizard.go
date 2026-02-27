package onboarding

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config represents the core settings gathered during onboarding
type Config struct {
	Model       string              `json:"model"`
	Provider    string              `json:"provider"`
	BaseURL     string              `json:"base_url,omitempty"`
	APIKey      string              `json:"api_key,omitempty"`
	ScriptsDir  string              `json:"scripts_dir"`
	Middlewares []MiddlewareSetting `json:"middlewares"`
}

// Wizard guide the user through the initial configuration of IRon
type Wizard struct {
	scanner *bufio.Scanner
}

func NewWizard() *Wizard {
	return &Wizard{
		scanner: bufio.NewScanner(os.Stdin),
	}
}

// Run starts the interactive setup process
func (w *Wizard) Run() (*Config, error) {
	fmt.Println("\n🚀 Welcome to IRon Onboarding!")
	fmt.Println("Let's configure your AI assistant and tools.")
	fmt.Println(strings.Repeat("-", 40))

	cfg := &Config{
		ScriptsDir: "scripts",
	}

	// 1. LLM Provider Setup
	fmt.Println("\n[1/3] LLM Configuration")
	w.askProvider(cfg)
	w.askModel(cfg)
	w.askBaseURL(cfg)
	w.askAPIKey(cfg)

	// 2. Tools & Scripts Setup
	fmt.Println("\n[2/3] Tools Configuration")
	fmt.Printf("Default scripts directory: %s\n", cfg.ScriptsDir)
	if _, err := os.Stat(cfg.ScriptsDir); os.IsNotExist(err) {
		fmt.Print("Directory does not exist. Create it? (Y/n): ")
		if w.confirm(true) {
			os.MkdirAll(cfg.ScriptsDir, 0755)
			fmt.Println("✅ Created scripts directory.")
		}
	}

	// 3. Google Credentials Check (Optional)
	fmt.Println("\n[3/3] External Integrations")
	fmt.Println("Some tools (Gmail/Calendar) require a 'credentials.json' file in the root directory.")
	if _, err := os.Stat("credentials.json"); os.IsNotExist(err) {
		fmt.Println("⚠️  'credentials.json' not found. You can add it later to enable Google tools.")
	} else {
		fmt.Println("✅ 'credentials.json' detected.")
	}

	// 4. Middleware Configuration
	menu := NewMiddlewareMenu(w.scanner)
	mSettings, err := menu.Run()
	if err == nil {
		cfg.Middlewares = mSettings
	}

	w.summarize(cfg)

	return cfg, nil
}

func (w *Wizard) askProvider(cfg *Config) {
	fmt.Println("Select LLM Provider:")
	fmt.Println("1) Ollama (Local)")
	fmt.Println("2) OpenAI")
	fmt.Println("3) Anthropic")
	fmt.Println("4) Gemini")

	for {
		fmt.Print("Choice (default: 1): ")
		w.scanner.Scan()
		input := strings.TrimSpace(w.scanner.Text())

		switch input {
		case "1", "":
			cfg.Provider = "ollama"
			return
		case "2":
			cfg.Provider = "openai"
			return
		case "3":
			cfg.Provider = "anthropic"
			return
		case "4":
			cfg.Provider = "gemini"
			return
		default:
			fmt.Println("❌ Invalid choice. Please select 1-4.")
		}
	}
}

func (w *Wizard) askModel(cfg *Config) {
	defaultModel := "llama3.2"
	if cfg.Provider == "openai" {
		defaultModel = "gpt-4o"
	} else if cfg.Provider == "anthropic" {
		defaultModel = "claude-3-5-sonnet-latest"
	}

	fmt.Printf("Enter Model Name (default: %s): ", defaultModel)
	w.scanner.Scan()
	input := strings.TrimSpace(w.scanner.Text())
	if input == "" {
		cfg.Model = defaultModel
	} else {
		cfg.Model = input
	}
}

func (w *Wizard) askBaseURL(cfg *Config) {
	if cfg.Provider != "ollama" && cfg.Provider != "gemini" {
		return
	}

	defaultURL := "http://localhost:11434"
	if cfg.Provider == "gemini" {
		defaultURL = "" // Uses default in adapter
	}

	fmt.Printf("Enter Base URL (press Enter for default %s): ", defaultURL)
	w.scanner.Scan()
	input := strings.TrimSpace(w.scanner.Text())
	if input == "" {
		cfg.BaseURL = defaultURL
	} else {
		cfg.BaseURL = input
	}
}

func (w *Wizard) askAPIKey(cfg *Config) {
	if cfg.Provider == "ollama" {
		return
	}

	envVar := strings.ToUpper("IRON_" + cfg.Provider + "_API_KEY")
	fmt.Printf("Enter API Key (or leave empty if set in %s): ", envVar)
	w.scanner.Scan()
	cfg.APIKey = strings.TrimSpace(w.scanner.Text())
}

func (w *Wizard) confirm(def bool) bool {
	w.scanner.Scan()
	input := strings.ToLower(strings.TrimSpace(w.scanner.Text()))
	if input == "" {
		return def
	}
	return input == "y" || input == "yes"
}

func (w *Wizard) summarize(cfg *Config) {
	fmt.Println("\n" + strings.Repeat("=", 40))
	fmt.Println("Setup Summary:")
	fmt.Printf("Provider: %s\n", cfg.Provider)
	fmt.Printf("Model:    %s\n", cfg.Model)
	if cfg.BaseURL != "" {
		fmt.Printf("URL:      %s\n", cfg.BaseURL)
	}
	fmt.Println(strings.Repeat("=", 40))

	fmt.Println("\nTip: You can save these as environment variables:")
	fmt.Printf("export IRON_PROVIDER=%s\n", cfg.Provider)
	fmt.Printf("export IRON_MODEL=%s\n", cfg.Model)
	if cfg.APIKey != "" {
		fmt.Printf("export IRON_%s_API_KEY=***\n", strings.ToUpper(cfg.Provider))
	}
	fmt.Println("\n✅ Configuration complete! Restart IRon to apply changes.")
}

// LoadFromFile loads the configuration from a JSON file
func LoadFromFile(path string) (*Config, error) {
	// Expand ~ to home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, path[2:])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
