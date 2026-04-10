package layouts

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Layout defines how the TUI arranges its content.
type Layout interface {
	tea.Model
	// Name returns the layout identifier.
	Name() string
}

var registry = map[string]func() Layout{}

// Register adds a layout factory to the registry.
func Register(name string, factory func() Layout) {
	registry[name] = factory
}

// Get returns a layout by name. Returns nil if not found.
func Get(name string) Layout {
	if f, ok := registry[name]; ok {
		return f()
	}
	return nil
}

// List returns all registered layout names.
func List() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}

// Default returns the default layout name.
func Default() string { return "single" }
