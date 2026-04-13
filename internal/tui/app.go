package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/config"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/commands"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/components"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/theme"
)

// activeView tracks which content is displayed.
type activeView int

const (
	viewSearch activeView = iota
	viewQueue
	viewHistory
	viewNowPlaying
)

func (v activeView) String() string {
	switch v {
	case viewSearch:
		return "search"
	case viewQueue:
		return "queue"
	case viewHistory:
		return "history"
	case viewNowPlaying:
		return "now playing"
	default:
		return "unknown"
	}
}

// App is the root Bubble Tea model.
type App struct {
	facade   *Facade
	dispatch *commands.Dispatcher
	prompt   components.Prompt
	toast    components.Toast
	statusBar components.StatusBar
	theme    theme.Theme
	styles   theme.Styles
	cfg      config.Config

	// View state
	currentView activeView
	width       int
	height      int

	// Search state
	searchResults []core.SearchResult
	searchQuery   string
	searchCursor  int

	// Queue cursor for navigation
	queueCursor int

	// History state
	historyEntries []core.HistoryEntry
	historyCursor  int

	// Playback position tracking
	currentPos float64
	currentDur float64

	// Auto-advance: play next track when current ends
	autoAdvance bool

	// Loading indicator
	loading     bool
	loadingText string

	// Help text display
	helpText string
}

// NewApp creates the root TUI application.
func NewApp(facade *Facade, cfg config.Config) *App {
	t := theme.Get(cfg.Theme)
	app := &App{
		facade:      facade,
		cfg:         cfg,
		theme:       t,
		styles:      t.Styles(),
		prompt:      components.NewPrompt(t),
		toast:       components.NewToast(t),
		statusBar:   components.NewStatusBar(t),
		autoAdvance: true,
	}
	app.dispatch = app.buildCommands()
	return app
}

func (a *App) buildCommands() *commands.Dispatcher {
	d := commands.NewDispatcher()

	d.Register(commands.Command{
		Name:        "/help",
		Description: "Show available commands",
		Handler: func(args string) (string, error) {
			a.helpText = d.Help() + "\nKeyboard shortcuts:\n" +
				"  Space          Toggle pause/resume\n" +
				"  Ctrl+P         Toggle pause (works while typing)\n" +
				"  Left/Right     Seek -/+ 5 seconds\n" +
				"  +/-            Volume up/down\n" +
				"  n/p            Next/previous track\n" +
				"  a              Add to queue (search view)\n" +
				"  Tab/Shift+Tab  Cycle views\n" +
				"  Up/Down        Navigate list\n" +
				"  Enter          Select/play item\n" +
				"  Esc            Back to search\n" +
				"  q              Quit\n"
			return "", nil
		},
	})

	d.Register(commands.Command{
		Name:        "/queue",
		Description: "Show the play queue",
		Handler: func(args string) (string, error) {
			a.currentView = viewQueue
			return "", nil
		},
	})

	d.Register(commands.Command{
		Name:        "/history",
		Description: "Show play history",
		Handler: func(args string) (string, error) {
			a.currentView = viewHistory
			return "", nil
		},
	})

	d.Register(commands.Command{
		Name:        "/now",
		Description: "Show now playing",
		Handler: func(args string) (string, error) {
			a.currentView = viewNowPlaying
			return "", nil
		},
	})

	d.Register(commands.Command{
		Name:        "/pause",
		Description: "Toggle pause/resume",
		Handler: func(args string) (string, error) {
			if err := a.facade.TogglePause(); err != nil {
				return "", err
			}
			return "", nil
		},
	})

	d.Register(commands.Command{
		Name:        "/stop",
		Description: "Stop playback",
		Handler: func(args string) (string, error) {
			if err := a.facade.Stop(); err != nil {
				return "", err
			}
			return "", nil
		},
	})

	d.Register(commands.Command{
		Name:        "/next",
		Description: "Play next track in queue",
		Handler: func(args string) (string, error) {
			return "NEXT_TRACK", nil
		},
	})

	d.Register(commands.Command{
		Name:        "/prev",
		Description: "Play previous track in queue",
		Handler: func(args string) (string, error) {
			return "PREV_TRACK", nil
		},
	})

	d.Register(commands.Command{
		Name:        "/vol",
		Description: "Set volume (0-100)",
		Handler: func(args string) (string, error) {
			var vol int
			if _, err := fmt.Sscanf(args, "%d", &vol); err != nil {
				return "", fmt.Errorf("usage: /vol <0-100>")
			}
			if err := a.facade.SetVolume(vol); err != nil {
				return "", err
			}
			a.cfg.Volume = a.facade.State().Volume
			_ = config.Save(a.cfg)
			return fmt.Sprintf("Volume: %d%%", vol), nil
		},
	})

	d.Register(commands.Command{
		Name:        "/theme",
		Description: "Switch theme",
		Handler: func(args string) (string, error) {
			args = strings.TrimSpace(args)
			if args == "" {
				return fmt.Sprintf("Current: %s\nAvailable: %s", a.theme.Name, strings.Join(theme.List(), ", ")), nil
			}
			a.theme = theme.Get(args)
			a.styles = a.theme.Styles()
			a.prompt = components.NewPrompt(a.theme)
			a.toast = components.NewToast(a.theme)
			a.statusBar = components.NewStatusBar(a.theme)
			// Persist theme choice
			a.cfg.Theme = a.theme.Name
			_ = config.Save(a.cfg)
			return fmt.Sprintf("Theme: %s (saved)", a.theme.Name), nil
		},
	})

	d.Register(commands.Command{
		Name:        "/clear",
		Description: "Clear the queue",
		Handler: func(args string) (string, error) {
			a.facade.Queue().Clear()
			return "Queue cleared", nil
		},
	})

	d.Register(commands.Command{
		Name:        "/search",
		Description: "Search YouTube",
		Handler: func(args string) (string, error) {
			if args == "" {
				return "", fmt.Errorf("usage: /search <query>")
			}
			a.currentView = viewSearch
			return "SEARCH:" + args, nil
		},
	})

	d.Register(commands.Command{
		Name:        "/repeat",
		Description: "Cycle repeat mode (off → all → one)",
		Handler: func(args string) (string, error) {
			q := a.facade.Queue()
			switch q.Repeat {
			case core.RepeatOff:
				q.Repeat = core.RepeatAll
			case core.RepeatAll:
				q.Repeat = core.RepeatOne
			case core.RepeatOne:
				q.Repeat = core.RepeatOff
			}
			return fmt.Sprintf("Repeat: %s", q.Repeat), nil
		},
	})

	d.Register(commands.Command{
		Name:        "/shuffle",
		Description: "Toggle shuffle mode",
		Handler: func(args string) (string, error) {
			q := a.facade.Queue()
			q.Shuffle = !q.Shuffle
			mode := "off"
			if q.Shuffle {
				mode = "on"
			}
			return fmt.Sprintf("Shuffle: %s", mode), nil
		},
	})

	d.Register(commands.Command{
		Name:        "/update-ytdlp",
		Description: "Update bundled yt-dlp",
		Handler: func(args string) (string, error) {
			return "UPDATE_YTDLP", nil
		},
	})

	// Reserve v2.x namespaces (hidden)
	for _, ns := range []string{"/playlist", "/lyrics", "/download", "/eq", "/focus", "/sleep"} {
		name := ns
		d.Register(commands.Command{
			Name:        name,
			Description: "Coming in v2.x",
			Hidden:      true,
			Handler: func(args string) (string, error) {
				return fmt.Sprintf("%s is coming in a future release", name), nil
			},
		})
	}

	return d
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	// Apply saved volume
	if a.cfg.Volume > 0 {
		_ = a.facade.SetVolume(a.cfg.Volume)
	}
	return textinput.Blink
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "q":
			if a.prompt.Value() == "" {
				return a, tea.Quit
			}
		case " ":
			// Space toggles pause when prompt is empty
			if a.prompt.Value() == "" {
				if err := a.facade.TogglePause(); err != nil {
					cmds = append(cmds, a.toast.Show(err.Error(), true))
				}
				a.statusBar.SetState(a.facade.State())
				return a, tea.Batch(cmds...)
			}
		case "ctrl+p":
			if err := a.facade.TogglePause(); err != nil {
				cmds = append(cmds, a.toast.Show(err.Error(), true))
			}
			a.statusBar.SetState(a.facade.State())
			return a, tea.Batch(cmds...)
		case "left":
			// Seek backward 5s when prompt is empty
			if a.prompt.Value() == "" {
				_ = a.facade.Seek(-5)
				return a, nil
			}
		case "right":
			// Seek forward 5s when prompt is empty
			if a.prompt.Value() == "" {
				_ = a.facade.Seek(5)
				return a, nil
			}
		case "+", "=":
			if a.prompt.Value() == "" {
				_ = a.facade.VolumeUp()
				a.statusBar.SetState(a.facade.State())
				cmds = append(cmds, a.toast.Show(fmt.Sprintf("Volume: %d%%", a.facade.State().Volume), false))
				return a, tea.Batch(cmds...)
			}
		case "-":
			if a.prompt.Value() == "" {
				_ = a.facade.VolumeDown()
				a.statusBar.SetState(a.facade.State())
				cmds = append(cmds, a.toast.Show(fmt.Sprintf("Volume: %d%%", a.facade.State().Volume), false))
				return a, tea.Batch(cmds...)
			}
		case "n":
			if a.prompt.Value() == "" {
				cmds = append(cmds, a.doNextTrack())
				return a, tea.Batch(cmds...)
			}
		case "p":
			if a.prompt.Value() == "" {
				cmds = append(cmds, a.doPrevTrack())
				return a, tea.Batch(cmds...)
			}
		case "up":
			if a.prompt.Value() == "" {
				a.moveCursorUp()
				return a, nil
			}
		case "down":
			if a.prompt.Value() == "" {
				a.moveCursorDown()
				return a, nil
			}
		case "enter":
			if a.prompt.Value() == "" {
				cmd := a.handleEnterOnList()
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				return a, tea.Batch(cmds...)
			}
		case "esc":
			// Clear help text or switch to search view
			if a.helpText != "" {
				a.helpText = ""
			} else {
				a.currentView = viewSearch
			}
			return a, nil
		case "tab":
			if a.prompt.Value() == "" {
				// Cycle views: search → queue → now playing → history → search
				switch a.currentView {
				case viewSearch:
					a.currentView = viewQueue
				case viewQueue:
					a.currentView = viewNowPlaying
				case viewNowPlaying:
					a.currentView = viewHistory
					a.loading = true
					cmds = append(cmds, a.doLoadHistory())
				case viewHistory:
					a.currentView = viewSearch
				}
				return a, tea.Batch(cmds...)
			}
		case "shift+tab":
			if a.prompt.Value() == "" {
				// Reverse cycle
				switch a.currentView {
				case viewSearch:
					a.currentView = viewHistory
					a.loading = true
					cmds = append(cmds, a.doLoadHistory())
				case viewQueue:
					a.currentView = viewSearch
				case viewNowPlaying:
					a.currentView = viewQueue
				case viewHistory:
					a.currentView = viewNowPlaying
				}
				return a, tea.Batch(cmds...)
			}
		case "a":
			// Add to queue without playing (search view only)
			if a.prompt.Value() == "" && a.currentView == viewSearch {
				if a.searchCursor >= 0 && a.searchCursor < len(a.searchResults) {
					result := a.searchResults[a.searchCursor]
					track := a.facade.AddToQueue(result)
					cmds = append(cmds, a.toast.Show("Queued: "+track.Title, false))
					return a, tea.Batch(cmds...)
				}
			}
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.statusBar.SetWidth(msg.Width)

	case components.PromptSubmitMsg:
		cmd := a.handleSubmit(msg.Value)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case SearchResultMsg:
		a.loading = false
		if msg.Err != nil {
			cmds = append(cmds, a.toast.Show("Search error: "+msg.Err.Error(), true))
		} else {
			a.searchResults = msg.Results
			a.searchQuery = msg.Query
			a.searchCursor = 0
			a.currentView = viewSearch
			if len(msg.Results) == 0 {
				cmds = append(cmds, a.toast.Show("No results found", true))
			}
		}

	case PlaybackStartedMsg:
		a.currentPos = 0
		a.currentDur = msg.Track.Duration.Seconds()
		a.statusBar.SetState(a.facade.State())
		cmds = append(cmds, a.toast.Show("Now playing: "+msg.Track.Title, false))
		cmds = append(cmds, a.tickPosition())

	case PlaybackStoppedMsg:
		a.statusBar.SetState(a.facade.State())

	case PlaybackErrorMsg:
		a.loading = false
		cmds = append(cmds, a.toast.Show("Playback error: "+msg.Err.Error(), true))

	case HistoryLoadedMsg:
		a.loading = false
		if msg.Err != nil {
			cmds = append(cmds, a.toast.Show("History error: "+msg.Err.Error(), true))
		} else {
			a.historyEntries = msg.Entries
			a.historyCursor = 0
		}

	case ToastMsg:
		cmds = append(cmds, a.toast.Show(msg.Text, msg.IsErr))

	case PositionUpdateMsg:
		a.currentPos = msg.Position
		if msg.Duration > 0 {
			a.currentDur = msg.Duration
		}
		if a.facade.State().Status == core.StatusPlaying || a.facade.State().Current != nil {
			cmds = append(cmds, a.tickPosition())
		}

	case TrackEndedMsg:
		// Track finished naturally — auto-advance if enabled
		if a.autoAdvance {
			_, hasNext := a.facade.Queue().Next()
			if hasNext {
				cmds = append(cmds, a.doPlayFromQueue())
			} else {
				// No more tracks — stop
				a.facade.Stop()
				a.statusBar.SetState(a.facade.State())
				a.currentPos = 0
				a.currentDur = 0
				cmds = append(cmds, a.toast.Show("Queue finished", false))
			}
		}

	case YtDlpUpdateMsg:
		a.loading = false
		if msg.Err != nil {
			cmds = append(cmds, a.toast.Show("yt-dlp update failed: "+msg.Err.Error(), true))
		} else {
			cmds = append(cmds, a.toast.Show("yt-dlp: "+msg.Output, false))
		}

	case StreamURLMsg:
		// Handled by PlayTrack in facade; this msg type exists for future use.
	}

	// Update sub-components.
	var cmd tea.Cmd
	a.prompt, cmd = a.prompt.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	a.toast, cmd = a.toast.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Keep status bar in sync.
	a.statusBar.SetState(a.facade.State())
	a.statusBar.SetView(a.currentView.String())
	a.statusBar.SetPosition(a.currentPos, a.currentDur)
	q := a.facade.Queue()
	a.statusBar.SetRepeatShuffle(q.Repeat.String(), q.Shuffle)

	return a, tea.Batch(cmds...)
}

// View implements tea.Model.
func (a *App) View() string {
	if a.width == 0 {
		return ""
	}

	// Calculate content height: total - statusbar(1) - prompt(~2) - toast
	promptView := a.prompt.View()
	statusView := a.statusBar.View()
	toastView := a.toast.View()

	promptHeight := lipgloss.Height(promptView)
	statusHeight := lipgloss.Height(statusView)
	toastHeight := 0
	if toastView != "" {
		toastHeight = lipgloss.Height(toastView)
	}

	contentHeight := a.height - promptHeight - statusHeight - toastHeight
	if contentHeight < 0 {
		contentHeight = 0
	}

	// Render content area.
	content := a.renderContent(contentHeight)

	// Stack everything vertically.
	var sections []string
	sections = append(sections, content)
	if toastView != "" {
		sections = append(sections, toastView)
	}
	sections = append(sections, statusView)
	sections = append(sections, promptView)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (a *App) renderContent(height int) string {
	var content string

	// Show help overlay if active
	if a.helpText != "" {
		content = "\n" + a.styles.Title.Render("  Help") + "\n\n" + a.styles.Muted.Render(a.helpText) + "\n\n" + a.styles.Muted.Render("  Press Esc to close")
		return lipgloss.NewStyle().Width(a.width).Height(height).Render(content)
	}

	switch a.currentView {
	case viewSearch:
		content = a.renderSearchView()
	case viewQueue:
		content = a.renderQueueView()
	case viewHistory:
		content = a.renderHistoryView()
	case viewNowPlaying:
		content = a.renderNowPlayingView()
	}

	// Pad or truncate to fill content area.
	return lipgloss.NewStyle().
		Width(a.width).
		Height(height).
		Render(content)
}

func (a *App) renderSearchView() string {
	if len(a.searchResults) == 0 && a.searchQuery == "" {
		if a.loading {
			return a.styles.Muted.Render("  Searching...")
		}
		// Welcome screen with ASCII art.
		logo := a.styles.Accent.Render(
			"              _                          \n" +
				" __      __ _ | | __ _ __   ___  _ __    \n" +
				" \\ \\ /\\ / /| '__|| |/ /| '_ \\ / _ \\| '_ \\   \n" +
				"  \\ V  V / | |   |   < | | | | (_) | | | |  \n" +
				"   \\_/\\_/  |_|   |_|\\_\\|_| |_|\\___/|_| |_|  \n")
		version := a.styles.Muted.Render("                   v2.0 — Go Edition")
		hint1 := a.styles.Muted.Render("  Type to search YouTube")
		hint2 := a.styles.Muted.Render("  /help for commands  •  Tab to switch views")
		hint3 := a.styles.Muted.Render("  Enter to play  •  a to add to queue")
		return "\n" + logo + "\n" + version + "\n\n" + hint1 + "\n" + hint2 + "\n" + hint3
	}

	if a.loading {
		return a.styles.Muted.Render(fmt.Sprintf("  Searching for \"%s\"...", a.searchQuery))
	}

	if len(a.searchResults) == 0 {
		return a.styles.Muted.Render(fmt.Sprintf("  No results for \"%s\"", a.searchQuery))
	}

	var b strings.Builder
	header := a.styles.Title.Render(fmt.Sprintf("  Search: \"%s\" (%d results)", a.searchQuery, len(a.searchResults)))
	b.WriteString(header)
	b.WriteString("\n\n")

	for i, r := range a.searchResults {
		cursor := "  "
		if i == a.searchCursor {
			cursor = a.styles.Accent.Render("> ")
		}

		num := a.styles.Muted.Render(fmt.Sprintf("%2d. ", i+1))
		dur := formatDuration(r.Duration)

		title := r.Title
		if i == a.searchCursor {
			title = a.styles.Selected.Render(title)
		}

		channel := a.styles.Muted.Render(" - " + r.Channel)
		duration := a.styles.Muted.Render(" [" + dur + "]")

		b.WriteString(cursor + num + title + channel + duration + "\n")
	}

	b.WriteString("\n")
	b.WriteString(a.styles.Muted.Render("  Enter: play  •  a: add to queue  •  Tab: switch view"))

	return b.String()
}

func (a *App) renderQueueView() string {
	tracks := a.facade.Queue().Tracks()

	if len(tracks) == 0 {
		return a.styles.Muted.Render("  Queue is empty. Search for music to get started.")
	}

	current, hasCurrentTrack := a.facade.Queue().Current()

	var b strings.Builder
	header := a.styles.Title.Render(fmt.Sprintf("  Queue (%d tracks)", len(tracks)))
	b.WriteString(header)
	b.WriteString("\n\n")

	for i, t := range tracks {
		cursor := "  "
		isCurrent := hasCurrentTrack && t.ID == current.ID

		if i == a.queueCursor {
			cursor = a.styles.Accent.Render("> ")
		}

		num := a.styles.Muted.Render(fmt.Sprintf("%2d. ", i+1))
		dur := formatDuration(t.Duration)

		title := t.Title
		if isCurrent {
			title = a.styles.Accent.Render("\u25b6 " + title)
		} else if i == a.queueCursor {
			title = a.styles.Selected.Render(title)
		}

		channel := a.styles.Muted.Render(" - " + t.Channel)
		duration := a.styles.Muted.Render(" [" + dur + "]")

		b.WriteString(cursor + num + title + channel + duration + "\n")
	}

	return b.String()
}

func (a *App) renderHistoryView() string {
	if len(a.historyEntries) == 0 {
		return a.styles.Muted.Render("  No history yet.")
	}

	var b strings.Builder
	header := a.styles.Title.Render(fmt.Sprintf("  History (%d entries)", len(a.historyEntries)))
	b.WriteString(header)
	b.WriteString("\n\n")

	for i, e := range a.historyEntries {
		cursor := "  "
		if i == a.historyCursor {
			cursor = a.styles.Accent.Render("> ")
		}

		num := a.styles.Muted.Render(fmt.Sprintf("%2d. ", i+1))
		title := e.Track.Title
		if i == a.historyCursor {
			title = a.styles.Selected.Render(title)
		}

		channel := a.styles.Muted.Render(" - " + e.Track.Channel)
		ago := a.styles.Muted.Render(" (" + timeAgo(e.PlayedAt) + ")")

		b.WriteString(cursor + num + title + channel + ago + "\n")
	}

	return b.String()
}

func (a *App) renderNowPlayingView() string {
	state := a.facade.State()
	if state.Current == nil {
		return a.styles.Muted.Render("  Nothing playing.\n\n  Search for music to get started.")
	}

	var b strings.Builder

	b.WriteString("\n")
	title := a.styles.Title.Render("  Now Playing")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Track info
	trackTitle := a.styles.Selected.Render("  " + state.Current.Title)
	b.WriteString(trackTitle)
	b.WriteString("\n")

	if state.Current.Channel != "" {
		channel := a.styles.Muted.Render("  " + state.Current.Channel)
		b.WriteString(channel)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Status icon
	status := a.styles.Accent.Render("  \u25b6 Playing")
	if state.Status == core.StatusPaused {
		status = a.styles.Warning.Render("  \u23f8 Paused")
	}
	b.WriteString(status)
	b.WriteString("\n\n")

	// Progress bar using actual position
	barWidth := a.width - 6
	if barWidth < 10 {
		barWidth = 10
	}
	bar := progressBar(a.currentPos, a.currentDur, barWidth)
	b.WriteString("  " + a.styles.Accent.Render(bar))
	b.WriteString("\n")

	// Time display
	posStr := formatSeconds(a.currentPos)
	durStr := formatSeconds(a.currentDur)
	timeDisplay := a.styles.Muted.Render(fmt.Sprintf("  %s / %s", posStr, durStr))
	b.WriteString(timeDisplay)
	b.WriteString("\n\n")

	// Volume bar
	volBar := volumeBar(state.Volume, 20)
	volDisplay := fmt.Sprintf("  Volume: %s %d%%", volBar, state.Volume)
	b.WriteString(a.styles.Muted.Render(volDisplay))
	b.WriteString("\n\n")

	// Keyboard shortcuts
	b.WriteString(a.styles.Muted.Render("  Space: pause  \u2190\u2192: seek  +/-: vol  n/p: track  Tab: views"))

	return b.String()
}

// handleSubmit processes user input from the prompt.
func (a *App) handleSubmit(input string) tea.Cmd {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	// Try slash command first.
	if strings.HasPrefix(input, "/") {
		result, handled, err := a.dispatch.Execute(input)
		if !handled {
			return a.toast.Show("Unknown command: "+input, true)
		}
		if err != nil {
			return a.toast.Show(err.Error(), true)
		}

		// Handle special return values.
		switch {
		case result == "NEXT_TRACK":
			return a.doNextTrack()
		case result == "PREV_TRACK":
			return a.doPrevTrack()
		case result == "UPDATE_YTDLP":
			return a.doUpdateYtDlp()
		case strings.HasPrefix(result, "SEARCH:"):
			query := strings.TrimPrefix(result, "SEARCH:")
			a.searchQuery = query
			a.loading = true
			a.loadingText = "Searching..."
			return a.doSearch(query)
		case result != "":
			return a.toast.Show(result, false)
		}

		// View-switching commands (/history) need to trigger data loading.
		if a.currentView == viewHistory {
			a.loading = true
			return a.doLoadHistory()
		}

		return nil
	}

	// Plain text = search query.
	a.searchQuery = input
	a.loading = true
	a.loadingText = "Searching..."
	a.currentView = viewSearch
	return a.doSearch(input)
}

// handleEnterOnList handles Enter key when the prompt is empty and a list is focused.
func (a *App) handleEnterOnList() tea.Cmd {
	switch a.currentView {
	case viewSearch:
		if a.searchCursor >= 0 && a.searchCursor < len(a.searchResults) {
			result := a.searchResults[a.searchCursor]
			track := a.facade.AddToQueue(result)
			return a.doPlayTrack(track)
		}
	case viewQueue:
		tracks := a.facade.Queue().Tracks()
		if a.queueCursor >= 0 && a.queueCursor < len(tracks) {
			track := tracks[a.queueCursor]
			return a.doPlayTrack(track)
		}
	case viewHistory:
		if a.historyCursor >= 0 && a.historyCursor < len(a.historyEntries) {
			entry := a.historyEntries[a.historyCursor]
			track := a.facade.AddToQueue(core.SearchResult{
				VideoID:  entry.Track.VideoID,
				Title:    entry.Track.Title,
				Channel:  entry.Track.Channel,
				Duration: entry.Track.Duration,
			})
			return a.doPlayTrack(track)
		}
	}
	return nil
}

func (a *App) moveCursorUp() {
	switch a.currentView {
	case viewSearch:
		if a.searchCursor > 0 {
			a.searchCursor--
		}
	case viewQueue:
		if a.queueCursor > 0 {
			a.queueCursor--
		}
	case viewHistory:
		if a.historyCursor > 0 {
			a.historyCursor--
		}
	}
}

func (a *App) moveCursorDown() {
	switch a.currentView {
	case viewSearch:
		if a.searchCursor < len(a.searchResults)-1 {
			a.searchCursor++
		}
	case viewQueue:
		tracks := a.facade.Queue().Tracks()
		if a.queueCursor < len(tracks)-1 {
			a.queueCursor++
		}
	case viewHistory:
		if a.historyCursor < len(a.historyEntries)-1 {
			a.historyCursor++
		}
	}
}

// --- Tea commands ---

func (a *App) doSearch(query string) tea.Cmd {
	limit := a.cfg.ResultsPerPage
	if limit <= 0 {
		limit = 10
	}
	return func() tea.Msg {
		results, err := a.facade.Search(context.Background(), query, limit)
		return SearchResultMsg{Results: results, Query: query, Err: err}
	}
}

func (a *App) doPlayTrack(track core.Track) tea.Cmd {
	return func() tea.Msg {
		err := a.facade.PlayTrack(context.Background(), track)
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		return PlaybackStartedMsg{Track: track}
	}
}

func (a *App) doPlayFromQueue() tea.Cmd {
	return func() tea.Msg {
		err := a.facade.PlayFromQueue(context.Background())
		if err != nil {
			return PlaybackErrorMsg{Err: err}
		}
		state := a.facade.State()
		if state.Current != nil {
			return PlaybackStartedMsg{Track: *state.Current}
		}
		return PlaybackStoppedMsg{}
	}
}

func (a *App) doNextTrack() tea.Cmd {
	return func() tea.Msg {
		err := a.facade.NextTrack(context.Background())
		if err != nil {
			return ToastMsg{Text: err.Error(), IsErr: true}
		}
		state := a.facade.State()
		if state.Current != nil {
			return PlaybackStartedMsg{Track: *state.Current}
		}
		return PlaybackStoppedMsg{}
	}
}

func (a *App) doPrevTrack() tea.Cmd {
	return func() tea.Msg {
		err := a.facade.PreviousTrack(context.Background())
		if err != nil {
			return ToastMsg{Text: err.Error(), IsErr: true}
		}
		state := a.facade.State()
		if state.Current != nil {
			return PlaybackStartedMsg{Track: *state.Current}
		}
		return PlaybackStoppedMsg{}
	}
}

func (a *App) doUpdateYtDlp() tea.Cmd {
	a.loading = true
	a.loadingText = "Updating yt-dlp..."
	return func() tea.Msg {
		output, err := a.facade.UpdateYtDlp(context.Background())
		return YtDlpUpdateMsg{Output: output, Err: err}
	}
}

func (a *App) doLoadHistory() tea.Cmd {
	return func() tea.Msg {
		entries, err := a.facade.LoadHistory(context.Background(), 50, 0)
		return HistoryLoadedMsg{Entries: entries, Err: err}
	}
}

func (a *App) tickPosition() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		// Check if mpv process died (track ended naturally)
		if a.facade.State().Current != nil && !a.facade.IsPlaying() {
			return TrackEndedMsg{}
		}
		pos, _ := a.facade.GetPosition()
		dur, _ := a.facade.GetDuration()
		return PositionUpdateMsg{Position: pos, Duration: dur}
	})
}

// --- Helpers ---

func progressBar(current, total float64, width int) string {
	if total <= 0 {
		return strings.Repeat("\u2500", width)
	}
	filled := int(current / total * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return strings.Repeat("\u2501", filled) + strings.Repeat("\u2500", width-filled)
}

func formatDuration(d time.Duration) string {
	total := int(d.Seconds())
	if total < 0 {
		total = 0
	}
	m := total / 60
	s := total % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

func volumeBar(vol int, width int) string {
	filled := vol * width / 100
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", width-filled)
}

func formatSeconds(secs float64) string {
	total := int(secs)
	if total < 0 {
		total = 0
	}
	m := total / 60
	s := total % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
