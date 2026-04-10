package components

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/theme"
)

type toastDoneMsg struct{}

// Toast displays a temporary notification.
type Toast struct {
	text    string
	isErr   bool
	visible bool
	styles  theme.Styles
	width   int
}

// NewToast creates a toast component.
func NewToast(t theme.Theme) Toast {
	return Toast{styles: t.Styles()}
}

// Init implements tea.Model.
func (t Toast) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (t Toast) Update(msg tea.Msg) (Toast, tea.Cmd) {
	switch msg := msg.(type) {
	case toastDoneMsg:
		t.visible = false
	case tea.WindowSizeMsg:
		t.width = msg.Width
	}
	return t, nil
}

// Show displays the toast with the given message.
func (t *Toast) Show(text string, isErr bool) tea.Cmd {
	t.text = text
	t.isErr = isErr
	t.visible = true
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return toastDoneMsg{}
	})
}

// View renders the toast.
func (t Toast) View() string {
	if !t.visible {
		return ""
	}
	style := lipgloss.NewStyle().
		Padding(0, 1).
		Width(t.width)

	if t.isErr {
		style = style.Foreground(t.styles.Error.GetForeground())
	} else {
		style = style.Foreground(t.styles.Success.GetForeground())
	}

	return style.Render(t.text)
}

// Visible returns whether the toast is currently showing.
func (t Toast) Visible() bool { return t.visible }
