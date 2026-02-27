package onboarding

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"iron/internal/middleware"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Styles ---

var (
	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	cursorStyle  = focusedStyle.Copy()
	noStyle      = lipgloss.NewStyle()
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	titleStyle   = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")).
			Padding(0, 1).
			Bold(true)

	docStyle = lipgloss.NewStyle().Padding(1, 2)

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("205")).
			Padding(0, 1)

	inactiveTabStyle = lipgloss.NewStyle().
				Padding(0, 1)

	windowStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1)
)

// --- Types ---

type state int

const (
	stateProvider state = iota
	stateModel
	stateAPIKey
	stateTelegram
	stateMiddlewares
	stateDone
)

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type TUIModel struct {
	state         state
	provider      string
	model         string
	apiKey        string
	telegramToken string
	baseURL       string
	middlewares   []MiddlewareSetting

	list     list.Model
	input    textinput.Model
	err      error
	quitting bool
	width    int
	height   int

	cursor   int // for middleware list
	tabIndex int
}

// --- Ollama Discovery ---

type ollamaModel struct {
	Name string `json:"name"`
}

type ollamaResponse struct {
	Models []ollamaModel `json:"models"`
}

func fetchOllamaModels() []item {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		return []item{{title: "llama3.2", desc: "Default fallback (Ollama not responding)"}}
	}
	defer resp.Body.Close()

	var data ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return []item{{title: "llama3.2", desc: "Error parsing models"}}
	}

	items := make([]item, len(data.Models))
	for i, m := range data.Models {
		items[i] = item{title: m.Name, desc: "Local Ollama model"}
	}
	return items
}

// --- Initial Model ---

func NewTUIModel() TUIModel {
	// Initialize list with providers
	providers := []list.Item{
		item{title: "ollama", desc: "Local execution via Ollama"},
		item{title: "openai", desc: "OpenAI GPT models (requires API Key)"},
		item{title: "anthropic", desc: "Claude models (requires API Key)"},
		item{title: "gemini", desc: "Google Gemini models (requires API Key)"},
	}

	l := list.New(providers, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select AI Provider"
	l.SetShowHelp(false)

	ti := textinput.New()
	ti.Placeholder = "Enter API Key"
	ti.Focus()

	// Initial middleware list
	mwList := middleware.Registered()
	settings := make([]MiddlewareSetting, len(mwList))
	for i, mw := range mwList {
		settings[i] = MiddlewareSetting{
			ID:      mw.ID(),
			Enabled: true,
			EnvVars: make(map[string]string),
		}
	}

	return TUIModel{
		state:       stateProvider,
		list:        l,
		input:       ti,
		middlewares: settings,
	}
}

func (m TUIModel) Init() tea.Cmd {
	return nil
}

func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.list.SetSize(msg.Width-10, msg.Height-15)
	}

	var cmd tea.Cmd

	switch m.state {
	case stateProvider:
		m.list, cmd = m.list.Update(msg)
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.provider = i.title
				if m.provider == "ollama" {
					m.state = stateModel
					m.baseURL = "http://localhost:11434"
					models := fetchOllamaModels()
					listItems := make([]list.Item, len(models))
					for i, item := range models {
						listItems[i] = item
					}
					m.list.SetItems(listItems)
					m.list.Title = "Select Local Model"
				} else {
					m.state = stateAPIKey
					m.input.Prompt = fmt.Sprintf("%s API Key: ", strings.Title(m.provider))
				}
			}
		}

	case stateAPIKey:
		m.input, cmd = m.input.Update(msg)
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			m.apiKey = m.input.Value()
			m.state = stateModel

			// Cloud defaults
			var models []list.Item
			if m.provider == "openai" {
				models = []list.Item{item{title: "gpt-4o", desc: "Best OpenAI model"}, item{title: "gpt-4o-mini", desc: "Fast OpenAI model"}}
			} else if m.provider == "anthropic" {
				models = []list.Item{item{title: "claude-3-5-sonnet-latest", desc: "Best Anthropic model"}}
			} else {
				models = []list.Item{item{title: "gemini-2.5-flash", desc: "Fast Google model"}, item{title: "gemini-2.5-pro", desc: "Powerful Google model"}}
			}
			m.list.SetItems(models)
			m.list.Title = "Select Cloud Model"
		}

	case stateModel:
		m.list, cmd = m.list.Update(msg)
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.model = i.title
				m.state = stateTelegram
				m.input.Prompt = "Telegram Bot Token (optional): "
				m.input.SetValue("")
			}
		}

	case stateTelegram:
		m.input, cmd = m.input.Update(msg)
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
			m.telegramToken = m.input.Value()
			m.state = stateMiddlewares
		}

	case stateMiddlewares:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.middlewares)-1 {
					m.cursor++
				}
			case " ":
				m.middlewares[m.cursor].Enabled = !m.middlewares[m.cursor].Enabled
			case "enter":
				m.state = stateDone
				return m, m.saveConfig()
			}
		}

	case stateDone:
		m.quitting = true
		return m, tea.Quit
	}

	return m, cmd
}

func (m TUIModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder
	s.WriteString(titleStyle.Render(" IRon Setup Wizard "))
	s.WriteString("\n\n")

	// Tabs logic (Visual progress)
	tabs := []string{"Provider", "Model", "Telegram", "Middlewares", "Finish"}
	var renderedTabs []string
	currentTab := int(m.state)
	if m.state == stateAPIKey {
		currentTab = 0
	} // API key is sub-step of provider
	if m.state > stateAPIKey {
		currentTab-- // Adjust for stateAPIKey shift
	}

	for i, t := range tabs {
		if i == currentTab {
			renderedTabs = append(renderedTabs, activeTabStyle.Render(t))
		} else {
			renderedTabs = append(renderedTabs, inactiveTabStyle.Render(t))
		}
	}
	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...))
	s.WriteString("\n\n")

	var content string
	switch m.state {
	case stateProvider, stateModel:
		content = m.list.View()
	case stateAPIKey, stateTelegram:
		content = "\n" + m.input.View() + "\n\n" + helpStyle.Render("Press enter to continue")
	case stateMiddlewares:
		var mwView strings.Builder
		mwView.WriteString("Toggle middlewares with [SPACE], Press [ENTER] to finish.\n\n")
		for i, mw := range m.middlewares {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
			}
			checked := " "
			if mw.Enabled {
				checked = "x"
			}
			line := fmt.Sprintf("%s [%s] %s", cursor, checked, mw.ID)
			if m.cursor == i {
				mwView.WriteString(focusedStyle.Render(line) + "\n")
			} else {
				mwView.WriteString(line + "\n")
			}
		}
		content = mwView.String()
	case stateDone:
		content = "\nSaving configuration to ~/.iron/config.json...\nDone! Press any key to exit."
	}

	s.WriteString(windowStyle.Width(m.width - 10).Height(m.height - 15).Render(content))

	if m.state != stateDone {
		s.WriteString("\n\n" + helpStyle.Render("q/ctrl+c: quit • ↑/↓: navigate • enter: select"))
	}

	return docStyle.Render(s.String())
}

func (m TUIModel) saveConfig() tea.Cmd {
	return func() tea.Msg {
		cfg := Config{
			Provider:    m.provider,
			Model:       m.model,
			APIKey:      m.apiKey,
			BaseURL:     m.baseURL,
			ScriptsDir:  "scripts",
			Middlewares: m.middlewares,
		}

		if m.telegramToken != "" {
			// Find slack/telegram middleware logic or just set an env var
			found := false
			for i, mw := range cfg.Middlewares {
				if mw.ID == "telegram" {
					if cfg.Middlewares[i].EnvVars == nil {
						cfg.Middlewares[i].EnvVars = make(map[string]string)
					}
					cfg.Middlewares[i].EnvVars["TELEGRAM_BOT_TOKEN"] = m.telegramToken
					found = true
					break
				}
			}
			if !found {
				// We inject it manually into an arbitrary persistent place
				// since the adapter reads os.Getenv directly.
				cfg.Middlewares = append(cfg.Middlewares, MiddlewareSetting{
					ID:      "telegram_bot",
					Enabled: true,
					EnvVars: map[string]string{
						"TELEGRAM_BOT_TOKEN": m.telegramToken,
					},
				})
			}
		}

		path := "~/.iron/config.json"
		if err := cfg.SaveToFile(path); err != nil {
			return err
		}
		return nil
	}
}

// --- Runner ---

func RunTUI() error {
	p := tea.NewProgram(NewTUIModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (cfg *Config) SaveToFile(path string) error {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = filepath.Join(home, path[2:])
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
