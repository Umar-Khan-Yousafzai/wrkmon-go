package components

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/theme"
)

// PromptSubmitMsg is sent when the user presses Enter.
type PromptSubmitMsg struct {
	Value string
}

// Prompt is the input field at the bottom of the TUI.
type Prompt struct {
	input  textinput.Model
	styles theme.Styles
	width  int
}

// NewPrompt creates a new prompt component.
func NewPrompt(t theme.Theme) Prompt {
	ti := textinput.New()
	ti.Prompt = "\u276f "
	ti.Placeholder = "Search or type /help..."
	ti.Focus()
	ti.CharLimit = 256

	return Prompt{
		input:  ti,
		styles: t.Styles(),
	}
}

// Init implements tea.Model.
func (p Prompt) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (p Prompt) Update(msg tea.Msg) (Prompt, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			val := p.input.Value()
			if val != "" {
				p.input.SetValue("")
				return p, func() tea.Msg {
					return PromptSubmitMsg{Value: val}
				}
			}
		}
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.input.Width = msg.Width - 4
	}

	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}

// View renders the prompt.
func (p Prompt) View() string {
	return lipgloss.NewStyle().
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(p.styles.Border.GetBorderBottomForeground()).
		Width(p.width).
		Render(p.input.View())
}

// Focus gives focus to the prompt input.
func (p *Prompt) Focus() tea.Cmd {
	return p.input.Focus()
}

// Blur removes focus from the prompt.
func (p *Prompt) Blur() {
	p.input.Blur()
}

// Value returns the current input value.
func (p Prompt) Value() string {
	return p.input.Value()
}
