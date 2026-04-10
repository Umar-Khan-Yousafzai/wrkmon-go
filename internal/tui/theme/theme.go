package theme

import "github.com/charmbracelet/lipgloss"

// Theme defines the color palette for the entire TUI.
type Theme struct {
	Name       string
	Background lipgloss.Color
	Foreground lipgloss.Color
	Accent     lipgloss.Color
	Muted      lipgloss.Color
	Border     lipgloss.Color
	Error      lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
}

// Styles returns pre-built lipgloss styles from this theme.
func (t Theme) Styles() Styles {
	return Styles{
		Base:      lipgloss.NewStyle().Background(t.Background).Foreground(t.Foreground),
		Title:     lipgloss.NewStyle().Foreground(t.Accent).Bold(true),
		Muted:     lipgloss.NewStyle().Foreground(t.Muted),
		Accent:    lipgloss.NewStyle().Foreground(t.Accent),
		Error:     lipgloss.NewStyle().Foreground(t.Error),
		Success:   lipgloss.NewStyle().Foreground(t.Success),
		Warning:   lipgloss.NewStyle().Foreground(t.Warning),
		Border:    lipgloss.NewStyle().BorderForeground(t.Border),
		Selected:  lipgloss.NewStyle().Foreground(t.Accent).Bold(true),
		StatusBar: lipgloss.NewStyle().Background(t.Accent).Foreground(t.Background).Padding(0, 1),
		Prompt:    lipgloss.NewStyle().Foreground(t.Accent),
	}
}

// Styles holds pre-built lipgloss styles.
type Styles struct {
	Base      lipgloss.Style
	Title     lipgloss.Style
	Muted     lipgloss.Style
	Accent    lipgloss.Style
	Error     lipgloss.Style
	Success   lipgloss.Style
	Warning   lipgloss.Style
	Border    lipgloss.Style
	Selected  lipgloss.Style
	StatusBar lipgloss.Style
	Prompt    lipgloss.Style
}
