package theme

import "github.com/charmbracelet/lipgloss"

var registry = map[string]Theme{}

func init() {
	Register(OpenCodeMono())
	Register(GitHubDark())
	Register(WarmMinimal())
}

// Register adds a theme to the registry.
func Register(t Theme) { registry[t.Name] = t }

// Get returns a theme by name. Falls back to OpenCode Mono.
func Get(name string) Theme {
	if t, ok := registry[name]; ok {
		return t
	}
	return registry["opencode-mono"]
}

// List returns all registered theme names.
func List() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}

// Default returns the default theme name.
func Default() string { return "opencode-mono" }

// === Built-in Themes ===

// OpenCodeMono is a near-grayscale theme with warm amber accent on near-black.
func OpenCodeMono() Theme {
	return Theme{
		Name:       "opencode-mono",
		Background: lipgloss.Color("#1a1a1a"),
		Foreground: lipgloss.Color("#d4d4d4"),
		Accent:     lipgloss.Color("#f5a623"),
		Muted:      lipgloss.Color("#666666"),
		Border:     lipgloss.Color("#333333"),
		Error:      lipgloss.Color("#e06c75"),
		Success:    lipgloss.Color("#98c379"),
		Warning:    lipgloss.Color("#e5c07b"),
	}
}

// GitHubDark uses multi-accent semantic colors like GitHub's dark theme.
func GitHubDark() Theme {
	return Theme{
		Name:       "github-dark",
		Background: lipgloss.Color("#0d1117"),
		Foreground: lipgloss.Color("#c9d1d9"),
		Accent:     lipgloss.Color("#58a6ff"),
		Muted:      lipgloss.Color("#8b949e"),
		Border:     lipgloss.Color("#30363d"),
		Error:      lipgloss.Color("#f85149"),
		Success:    lipgloss.Color("#3fb950"),
		Warning:    lipgloss.Color("#d29922"),
	}
}

// WarmMinimal is an espresso/cream/rose palette.
func WarmMinimal() Theme {
	return Theme{
		Name:       "warm-minimal",
		Background: lipgloss.Color("#1e1916"),
		Foreground: lipgloss.Color("#e8d5c4"),
		Accent:     lipgloss.Color("#e8a0bf"),
		Muted:      lipgloss.Color("#8a7b6b"),
		Border:     lipgloss.Color("#3d3229"),
		Error:      lipgloss.Color("#e06c75"),
		Success:    lipgloss.Color("#a8c97f"),
		Warning:    lipgloss.Color("#d4a857"),
	}
}
