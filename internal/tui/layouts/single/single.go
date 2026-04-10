package single

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/layouts"
)

// Layout implements the single-pane layout.
type Layout struct{}

func init() {
	layouts.Register("single", func() layouts.Layout {
		return &Layout{}
	})
}

func (l *Layout) Name() string                            { return "single" }
func (l *Layout) Init() tea.Cmd                           { return nil }
func (l *Layout) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return l, nil }
func (l *Layout) View() string                            { return "" }
