package skills

import (
	"context"
)

// Skill defines a tool that the LLM can use.
type Skill interface {
	// Name returns the unique name of the skill (e.g. "shell", "file_read").
	Name() string
	// Description returns a human-readable description for the LLM.
	Description() string
	// Parameters returns the JSON schema for the arguments as a map.
	Parameters() map[string]any
	// Execute runs the skill with the given arguments.
	Execute(ctx context.Context, args map[string]any) (string, error)
}

// Manager holds the available skills.
type Manager struct {
	skills map[string]Skill
}

func NewManager() *Manager {
	return &Manager{
		skills: make(map[string]Skill),
	}
}

func (m *Manager) Register(s Skill) {
	m.skills[s.Name()] = s
}

func (m *Manager) Get(name string) (Skill, bool) {
	s, ok := m.skills[name]
	return s, ok
}

func (m *Manager) List() []Skill {
	list := make([]Skill, 0, len(m.skills))
	for _, s := range m.skills {
		list = append(list, s)
	}
	return list
}
