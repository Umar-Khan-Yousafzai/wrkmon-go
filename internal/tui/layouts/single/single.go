package single

import (
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/layouts"
	tea "github.com/charmbracelet/bubbletea"
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
