package tui

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/config"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/commands"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/components"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/focus"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/theme"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// activeView tracks which content is displayed.
type activeView int

const (
	viewSearch activeView = iota
	viewQueue
	viewHistory
	viewNowPlaying
	viewPlaylists
	viewPlaylistDetail
	viewDownloads
)

// searchBatch is how many additional results each infinite-scroll fetch requests.
const searchBatch = 25

// npBarIndent is the left indent (in columns) of the now-playing progress
// bar line. It must match the "  " prefix written before the bar in
// renderNowPlayingView AND the a.npBarStart geometry captured there, since
// mouse-click hit testing (handleMouseMsg) uses npBarStart to map clicks
// back to the bar.
const npBarIndent = 2

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
	case viewPlaylists:
		return "playlists"
	case viewPlaylistDetail:
		return "playlist"
	case viewDownloads:
		return "downloads"
	default:
		return "unknown"
	}
}

// App is the root Bubble Tea model.
type App struct {
	facade    *Facade
	dispatch  *commands.Dispatcher
	prompt    components.Prompt
	toast     components.Toast
	statusBar components.StatusBar
	theme     theme.Theme
	styles    theme.Styles
	cfg       config.Config

	// View state
	currentView activeView
	width       int
	height      int

	// Search state
	searchResults []core.SearchResult
	searchQuery   string
	searchCursor  int

	searchExhausted   bool // no more results available
	searchLoadingMore bool // a refetch is in flight
	searchFetchFailed bool // last refetch failed; retry on next bottom gesture

	// Queue cursor for navigation
	queueCursor int

	// History state
	historyEntries []core.HistoryEntry
	historyCursor  int

	// Playlist state
	playlists       []core.Playlist
	playlistCursor  int
	currentPlaylist core.Playlist
	plDetailCursor  int

	// Downloads state
	downloads      []core.Download
	downloadCursor int

	// Lyrics state
	lyricsText  string
	lyricsTitle string

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

	// One-time seek hint, shown on first playback of the session
	seekHintShown bool

	// Seekbar drag state
	seekDragging bool
	lastDragSeek time.Time
	dragStart    int
	dragWidth    int

	// Now-playing big bar geometry, captured at render time
	npBarRow   int
	npBarStart int
	npBarWidth int

	// Equalizer state (persisted; see /eq command, saveEQ, eqStatusLine)
	eq core.EQState

	// Focus overlay state: /focus swaps the whole screen for a fake
	// "work" render (see internal/tui/focus) until any key is pressed.
	// While focusActive, View() renders ONLY the overlay and Update
	// intercepts all keys/mouse before the normal handling below.
	focusActive bool
	focusKind   focus.Kind
	focusTick   int
	focusSeed   int64
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
				"  Shift+←/→      Seek -/+ 30 seconds\n" +
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
		Name:        "/seek",
		Description: "Seek: /seek 1:23 | 83 | 50% | +30 | -30",
		Handler: func(args string) (string, error) {
			return a.execSeek(args)
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

	d.Register(commands.Command{
		Name:        "/playlist",
		Description: "Manage playlists: /playlist [create|play|delete|add] [args]",
		Handler: func(args string) (string, error) {
			args = strings.TrimSpace(args)
			if args == "" {
				// Show playlists view
				a.currentView = viewPlaylists
				return "LOAD_PLAYLISTS", nil
			}
			parts := strings.SplitN(args, " ", 2)
			sub := parts[0]
			subArgs := ""
			if len(parts) > 1 {
				subArgs = strings.TrimSpace(parts[1])
			}

			switch sub {
			case "create":
				if subArgs == "" {
					return "", fmt.Errorf("usage: /playlist create <name>")
				}
				return "PL_CREATE:" + subArgs, nil
			case "play":
				if subArgs == "" {
					return "", fmt.Errorf("usage: /playlist play <id>")
				}
				return "PL_PLAY:" + subArgs, nil
			case "delete":
				if subArgs == "" {
					return "", fmt.Errorf("usage: /playlist delete <id>")
				}
				return "PL_DELETE:" + subArgs, nil
			case "add":
				if subArgs == "" {
					return "", fmt.Errorf("usage: /playlist add <playlist_id>")
				}
				return "PL_ADD:" + subArgs, nil
			default:
				return "", fmt.Errorf("unknown: /playlist %s. Use create|play|delete|add", sub)
			}
		},
	})

	d.Register(commands.Command{
		Name:        "/lyrics",
		Description: "Show lyrics for current track",
		Handler: func(args string) (string, error) {
			state := a.facade.State()
			if state.Current == nil {
				return "", fmt.Errorf("nothing playing")
			}
			return "FETCH_LYRICS", nil
		},
	})

	d.Register(commands.Command{
		Name:        "/download",
		Description: "Download current track as MP3, or /download list",
		Handler: func(args string) (string, error) {
			args = strings.TrimSpace(args)
			if args == "list" {
				a.currentView = viewDownloads
				return "LOAD_DOWNLOADS", nil
			}
			// Download currently playing track
			state := a.facade.State()
			if state.Current == nil {
				return "", fmt.Errorf("nothing playing to download")
			}
			return "DOWNLOAD:" + state.Current.VideoID + "|" + state.Current.Title + "|" + state.Current.Channel, nil
		},
	})

	d.Register(commands.Command{
		Name:        "/eq",
		Description: "Equalizer: /eq <preset>|off|show|band <1-18> <dB>",
		Handler: func(args string) (string, error) {
			fields := strings.Fields(args)
			switch {
			case len(fields) == 0 || fields[0] == "show":
				return a.eqStatusLine(), nil
			case fields[0] == "off":
				a.eq.Enabled = false
				if err := a.facade.SetAudioFilter(""); err != nil {
					return "", err
				}
				a.saveEQ()
				return "EQ off", nil
			case fields[0] == "band" && len(fields) == 3:
				n, err1 := strconv.Atoi(fields[1])
				db, err2 := strconv.ParseFloat(fields[2], 64)
				if err1 != nil || err2 != nil {
					return "", fmt.Errorf("usage: /eq band <1-18> <-12..12>")
				}
				if err := a.eq.SetBand(n, db); err != nil {
					return "", err
				}
				if err := a.facade.SetAudioFilter(a.eq.FilterString()); err != nil {
					return "", err
				}
				a.saveEQ()
				return fmt.Sprintf("EQ band %d → %+.1f dB (custom)", n, db), nil
			default:
				e, ok := core.EQPreset(fields[0])
				if !ok {
					return "", fmt.Errorf("unknown preset %q (have: %s, off, band)", fields[0], strings.Join(core.EQPresetNames, ", "))
				}
				a.eq = e
				if err := a.facade.SetAudioFilter(a.eq.FilterString()); err != nil {
					return "", err
				}
				a.saveEQ()
				return "EQ preset: " + fields[0], nil
			}
		},
	})

	d.Register(commands.Command{
		Name:        "/focus",
		Description: "Show a fake work screen; any key returns",
		Handler: func(args string) (string, error) {
			a.focusSeed = time.Now().UnixNano()
			a.focusKind = focus.RandomKind(rand.New(rand.NewSource(a.focusSeed)))
			a.focusTick = 0
			a.focusActive = true
			return "FOCUS_ON", nil
		},
	})

	return d
}

// eqStatusLine renders the current EQ state for /eq and /eq show: the
// active preset plus a comma-separated list of non-zero bands (e.g.
// "3:+4.5dB"), or "EQ off" when disabled.
func (a *App) eqStatusLine() string {
	if !a.eq.Enabled {
		return "EQ off"
	}
	var bands []string
	for i, db := range a.eq.Gains {
		if db != 0 {
			bands = append(bands, fmt.Sprintf("%d:%+.1fdB", i+1, db))
		}
	}
	if len(bands) == 0 {
		return fmt.Sprintf("EQ preset: %s (flat)", a.eq.Preset)
	}
	return fmt.Sprintf("EQ preset: %s (%s)", a.eq.Preset, strings.Join(bands, ", "))
}

// saveEQ persists the current EQ state into config, mirroring how /vol and
// /theme persist their settings.
func (a *App) saveEQ() {
	gains := a.eq.Gains // array value copy, independent of a.eq
	a.cfg.EQPreset = a.eq.Preset
	a.cfg.EQGains = gains[:]
	a.cfg.EQEnabled = a.eq.Enabled
	_ = config.Save(a.cfg)
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	// Apply saved volume
	if a.cfg.Volume > 0 {
		_ = a.facade.SetVolume(a.cfg.Volume)
	}
	// Apply saved EQ
	if a.cfg.EQEnabled {
		a.eq = a.cfg.EQState()
		_ = a.facade.SetAudioFilter(a.eq.FilterString())
	}
	cmds := []tea.Cmd{textinput.Blink}
	if a.cfg.AutoUpdateYtDlp {
		cmds = append(cmds, a.doAutoUpdateYtDlp())
	}
	return tea.Batch(cmds...)
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseMsg:
		// The /focus overlay hijacks the whole screen; swallow mouse
		// events entirely while it's up rather than let them seek/click
		// through to the (invisible) player UI underneath.
		if a.focusActive {
			return a, nil
		}
		if cmd := a.handleMouse(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)

	case tea.KeyMsg:
		// The /focus overlay eats every key except ctrl+c (which still
		// quits the whole app, same as normal): any other key dismisses
		// the overlay and nothing else, before any of the usual key
		// handling below runs.
		if a.focusActive {
			if msg.String() == "ctrl+c" {
				return a, tea.Quit
			}
			a.focusActive = false
			return a, nil
		}
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
				cmds = append(cmds, a.seekRelativeKey(-5))
				return a, tea.Batch(cmds...)
			}
		case "right":
			// Seek forward 5s when prompt is empty
			if a.prompt.Value() == "" {
				cmds = append(cmds, a.seekRelativeKey(5))
				return a, tea.Batch(cmds...)
			}
		case "shift+left":
			cmds = append(cmds, a.seekRelativeKey(-30))
			return a, tea.Batch(cmds...)
		case "shift+right":
			cmds = append(cmds, a.seekRelativeKey(30))
			return a, tea.Batch(cmds...)
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
				if cmd := a.maybeLoadMore(); cmd != nil {
					cmds = append(cmds, cmd)
				}
				return a, tea.Batch(cmds...)
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
			// Clear overlays, go back from detail, or switch to search
			if a.helpText != "" {
				a.helpText = ""
			} else if a.lyricsText != "" {
				a.lyricsText = ""
				a.lyricsTitle = ""
			} else if a.currentView == viewPlaylistDetail {
				a.currentView = viewPlaylists
			} else {
				a.currentView = viewSearch
			}
			return a, nil
		case "tab":
			if a.prompt.Value() == "" {
				// Cycle views: search → queue → now playing → playlists → history → search
				switch a.currentView {
				case viewSearch:
					a.currentView = viewQueue
				case viewQueue:
					a.currentView = viewNowPlaying
				case viewNowPlaying:
					a.currentView = viewPlaylists
					cmds = append(cmds, a.doLoadPlaylists())
				case viewPlaylists, viewPlaylistDetail:
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
				case viewPlaylists, viewPlaylistDetail:
					a.currentView = viewNowPlaying
				case viewHistory:
					a.currentView = viewPlaylists
					cmds = append(cmds, a.doLoadPlaylists())
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

	case focusTickMsg:
		// A stray tick can arrive after the overlay was already dismissed
		// (the previous tick's timer was already in flight); ignore it and,
		// crucially, don't reschedule another one.
		if a.focusActive {
			a.focusTick++
			return a, focusTick()
		}
		return a, nil

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
			a.searchExhausted = false
			a.searchLoadingMore = false
			a.searchFetchFailed = false
			if len(msg.Results) == 0 {
				cmds = append(cmds, a.toast.Show("No results found", true))
			}
		}

	case SearchMoreMsg:
		a.searchLoadingMore = false
		if msg.Query == a.searchQuery { // ignore stale responses
			if msg.Err != nil {
				a.searchFetchFailed = true
				cmds = append(cmds, a.toast.Show("Load more failed: "+msg.Err.Error(), true))
			} else {
				merged, added := core.MergeResults(a.searchResults, msg.Results)
				a.searchResults = merged
				if added == 0 {
					a.searchExhausted = true
				} else {
					a.facade.CacheSearch(context.Background(), a.searchQuery, merged)
				}
				max := a.cfg.MaxSearchResults
				if max <= 0 {
					max = 100
				}
				if len(merged) >= max {
					a.searchExhausted = true
				}
			}
		}

	case PlaybackStartedMsg:
		a.currentPos = 0
		a.currentDur = msg.Track.Duration.Seconds()
		a.statusBar.SetState(a.facade.State())
		cmds = append(cmds, a.toast.Show("Now playing: "+msg.Track.Title, false))
		if !a.seekHintShown {
			a.seekHintShown = true
			cmds = append(cmds, a.toast.Show("Seek: ←/→ 5s · Shift ±30s · /seek 1:23 · click the bar", false))
		}
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

	case YtDlpAutoUpdateMsg:
		if msg.Err != nil {
			cmds = append(cmds, a.toast.Show("yt-dlp auto-update: "+msg.Err.Error(), true))
		} else if msg.Updated {
			cmds = append(cmds, a.toast.Show(msg.Info, false))
		}
		// silent when already current

	case PlaylistsLoadedMsg:
		a.loading = false
		if msg.Err != nil {
			cmds = append(cmds, a.toast.Show("Playlists error: "+msg.Err.Error(), true))
		} else {
			a.playlists = msg.Playlists
			a.playlistCursor = 0
		}

	case PlaylistDetailMsg:
		a.loading = false
		if msg.Err != nil {
			cmds = append(cmds, a.toast.Show(msg.Err.Error(), true))
		} else {
			a.currentPlaylist = msg.Playlist
			a.plDetailCursor = 0
			a.currentView = viewPlaylistDetail
		}

	case LyricsLoadedMsg:
		a.loading = false
		if msg.Err != nil {
			cmds = append(cmds, a.toast.Show("Lyrics: "+msg.Err.Error(), true))
		} else {
			a.lyricsText = msg.Lyrics
			a.lyricsTitle = msg.Title
		}

	case DownloadCompleteMsg:
		a.loading = false
		if msg.Err != nil {
			cmds = append(cmds, a.toast.Show("Download failed: "+msg.Err.Error(), true))
		} else {
			cmds = append(cmds, a.toast.Show("Downloaded: "+msg.Title, false))
		}

	case DownloadsLoadedMsg:
		a.loading = false
		if msg.Err != nil {
			cmds = append(cmds, a.toast.Show(msg.Err.Error(), true))
		} else {
			a.downloads = msg.Downloads
			a.downloadCursor = 0
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
	if a.focusActive {
		return focus.Render(a.focusKind, a.width, a.height, rand.New(rand.NewSource(a.focusSeed)), a.focusTick)
	}

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

	// Show lyrics overlay if active
	if a.lyricsText != "" {
		content = "\n" + a.styles.Title.Render("  Lyrics — "+a.lyricsTitle) + "\n\n" + a.styles.Muted.Render(a.lyricsText) + "\n\n" + a.styles.Muted.Render("  Press Esc to close")
		return lipgloss.NewStyle().Width(a.width).Height(height).Render(content)
	}

	switch a.currentView {
	case viewSearch:
		content = a.renderSearchView(height)
	case viewQueue:
		content = a.renderQueueView()
	case viewHistory:
		content = a.renderHistoryView()
	case viewNowPlaying:
		content = a.renderNowPlayingView()
	case viewPlaylists:
		content = a.renderPlaylistsView()
	case viewPlaylistDetail:
		content = a.renderPlaylistDetailView()
	case viewDownloads:
		content = a.renderDownloadsView()
	}

	// Pad or truncate to fill content area.
	return lipgloss.NewStyle().
		Width(a.width).
		Height(height).
		Render(content)
}

func (a *App) renderSearchView(height int) string {
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

	// Window the list so the cursor stays visible: header block above uses
	// 2 rows, the footer hint 2, the sentinel 1.
	listRows := height - 5
	if listRows < 3 {
		listRows = 3
	}
	start, end := core.VisibleRange(a.searchCursor, len(a.searchResults), listRows)
	for i := start; i < end; i++ {
		r := a.searchResults[i]
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

	// Sentinel row: fetch status at the bottom of the loaded list.
	switch {
	case a.searchLoadingMore:
		b.WriteString(a.styles.Muted.Render("     … loading more") + "\n")
	case a.searchFetchFailed:
		b.WriteString(a.styles.Warning.Render("     fetch failed — scroll to retry") + "\n")
	case a.searchExhausted && end == len(a.searchResults):
		b.WriteString(a.styles.Muted.Render("     — end of results —") + "\n")
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

// truncateLine shortens s so a line with the given prefix width never wraps.
func truncateLine(s string, max int) string {
	if max < 1 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func (a *App) renderNowPlayingView() string {
	state := a.facade.State()
	if state.Current == nil {
		a.npBarWidth = 0
		return a.styles.Muted.Render("  Nothing playing.\n\n  Search for music to get started.")
	}

	var b strings.Builder

	b.WriteString("\n")
	title := a.styles.Title.Render("  Now Playing")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Track info. Truncate so a long title/channel never wraps onto an
	// extra visual row — renderContent wraps at a.width, but npBarRow
	// below counts raw newlines and can't see wrapped rows.
	titleMax := a.width - 4
	trackTitle := a.styles.Selected.Render("  " + truncateLine(state.Current.Title, titleMax))
	b.WriteString(trackTitle)
	b.WriteString("\n")

	if state.Current.Channel != "" {
		channel := a.styles.Muted.Render("  " + truncateLine(state.Current.Channel, titleMax))
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
	a.npBarRow = strings.Count(b.String(), "\n") // row index of the bar line
	a.npBarStart = npBarIndent
	a.npBarWidth = barWidth
	b.WriteString(strings.Repeat(" ", npBarIndent) + a.styles.Accent.Render(bar))
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

func (a *App) renderDownloadsView() string {
	if len(a.downloads) == 0 {
		return a.styles.Muted.Render("  No downloads yet.\n\n  Play a track, then /download to save it as MP3.")
	}

	var b strings.Builder
	header := a.styles.Title.Render(fmt.Sprintf("  Downloads (%d)", len(a.downloads)))
	b.WriteString(header)
	b.WriteString("\n\n")

	for i, d := range a.downloads {
		cursor := "  "
		if i == a.downloadCursor {
			cursor = a.styles.Accent.Render("> ")
		}
		num := a.styles.Muted.Render(fmt.Sprintf("%2d. ", i+1))
		title := d.Title
		if i == a.downloadCursor {
			title = a.styles.Selected.Render(title)
		}
		channel := a.styles.Muted.Render(" - " + d.Channel)
		ago := a.styles.Muted.Render(" (" + timeAgo(d.DownloadedAt) + ")")
		b.WriteString(cursor + num + title + channel + ago + "\n")
	}

	return b.String()
}

func (a *App) renderPlaylistsView() string {
	if len(a.playlists) == 0 {
		return a.styles.Muted.Render("  No playlists yet.\n\n  /playlist create <name> to create one.")
	}

	var b strings.Builder
	header := a.styles.Title.Render(fmt.Sprintf("  Playlists (%d)", len(a.playlists)))
	b.WriteString(header)
	b.WriteString("\n\n")

	for i, p := range a.playlists {
		cursor := "  "
		if i == a.playlistCursor {
			cursor = a.styles.Accent.Render("> ")
		}
		num := a.styles.Muted.Render(fmt.Sprintf("%2d. ", i+1))
		name := p.Name
		if i == a.playlistCursor {
			name = a.styles.Selected.Render(name)
		}
		id := a.styles.Muted.Render(fmt.Sprintf(" (id:%d)", p.ID))
		b.WriteString(cursor + num + name + id + "\n")
	}

	b.WriteString("\n")
	b.WriteString(a.styles.Muted.Render("  Enter: open  •  /playlist play <id>  •  /playlist delete <id>"))

	return b.String()
}

func (a *App) renderPlaylistDetailView() string {
	p := a.currentPlaylist
	if len(p.Tracks) == 0 {
		return a.styles.Title.Render(fmt.Sprintf("  %s", p.Name)) + "\n\n" +
			a.styles.Muted.Render("  Empty playlist. Play a track, then /playlist add "+fmt.Sprintf("%d", p.ID))
	}

	var b strings.Builder
	header := a.styles.Title.Render(fmt.Sprintf("  %s (%d tracks)", p.Name, len(p.Tracks)))
	b.WriteString(header)
	b.WriteString("\n\n")

	for i, t := range p.Tracks {
		cursor := "  "
		if i == a.plDetailCursor {
			cursor = a.styles.Accent.Render("> ")
		}
		num := a.styles.Muted.Render(fmt.Sprintf("%2d. ", i+1))
		dur := formatDuration(t.Duration)

		title := t.Title
		if i == a.plDetailCursor {
			title = a.styles.Selected.Render(title)
		}
		channel := a.styles.Muted.Render(" - " + t.Channel)
		duration := a.styles.Muted.Render(" [" + dur + "]")
		b.WriteString(cursor + num + title + channel + duration + "\n")
	}

	b.WriteString("\n")
	b.WriteString(a.styles.Muted.Render(fmt.Sprintf("  Enter: play  •  /playlist play %d (play all)  •  Esc: back", p.ID)))

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
		case result == "FETCH_LYRICS":
			return a.doFetchLyrics()
		case result == "LOAD_DOWNLOADS":
			return a.doLoadDownloads()
		case result == "FOCUS_ON":
			return focusTick()
		case strings.HasPrefix(result, "DOWNLOAD:"):
			parts := strings.SplitN(strings.TrimPrefix(result, "DOWNLOAD:"), "|", 3)
			if len(parts) == 3 {
				return a.doDownloadTrack(parts[0], parts[1], parts[2])
			}
		case result == "LOAD_PLAYLISTS":
			return a.doLoadPlaylists()
		case strings.HasPrefix(result, "PL_CREATE:"):
			name := strings.TrimPrefix(result, "PL_CREATE:")
			return a.doCreatePlaylist(name)
		case strings.HasPrefix(result, "PL_PLAY:"):
			idStr := strings.TrimPrefix(result, "PL_PLAY:")
			var id int
			fmt.Sscanf(idStr, "%d", &id)
			return a.doPlayPlaylist(id)
		case strings.HasPrefix(result, "PL_DELETE:"):
			idStr := strings.TrimPrefix(result, "PL_DELETE:")
			var id int
			fmt.Sscanf(idStr, "%d", &id)
			return a.doDeletePlaylist(id)
		case strings.HasPrefix(result, "PL_ADD:"):
			idStr := strings.TrimPrefix(result, "PL_ADD:")
			var id int
			fmt.Sscanf(idStr, "%d", &id)
			return a.doAddToPlaylist(id)
		case strings.HasPrefix(result, "SEARCH:"):
			query := strings.TrimPrefix(result, "SEARCH:")
			a.searchQuery = query
			a.loading = true
			a.loadingText = "Searching..."
			return a.doSearch(query)
		case result != "":
			return a.toast.Show(result, false)
		}

		// View-switching commands need to trigger data loading.
		if a.currentView == viewHistory {
			a.loading = true
			return a.doLoadHistory()
		}
		if a.currentView == viewPlaylists {
			return a.doLoadPlaylists()
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
	case viewPlaylists:
		// Enter on playlist list = open detail view
		if a.playlistCursor >= 0 && a.playlistCursor < len(a.playlists) {
			return a.doLoadPlaylistDetail(a.playlists[a.playlistCursor].ID)
		}
	case viewPlaylistDetail:
		// Enter on playlist track = play it
		if a.plDetailCursor >= 0 && a.plDetailCursor < len(a.currentPlaylist.Tracks) {
			t := a.currentPlaylist.Tracks[a.plDetailCursor]
			track := a.facade.AddToQueue(core.SearchResult{
				VideoID:  t.VideoID,
				Title:    t.Title,
				Channel:  t.Channel,
				Duration: t.Duration,
			})
			return a.doPlayTrack(track)
		}
	}
	return nil
}

// execSeek applies a parsed /seek argument and returns the toast text.
func (a *App) execSeek(args string) (string, error) {
	if a.facade.State().Current == nil {
		return "", fmt.Errorf("nothing playing")
	}
	spec, err := commands.ParseSeek(args)
	if err != nil {
		return "", fmt.Errorf("usage: /seek 1:23 | 83 | 50%% | +30 | -30 (%v)", err)
	}
	var target float64
	switch spec.Kind {
	case commands.SeekAbsolute:
		target = spec.Value
		err = a.facade.SeekTo(spec.Value)
	case commands.SeekPct:
		target = spec.Value / 100 * a.currentDur
		err = a.facade.SeekPercent(spec.Value)
	case commands.SeekRelative:
		target = a.currentPos + spec.Value
		err = a.facade.Seek(spec.Value)
	}
	if err != nil {
		return "", err
	}
	a.currentPos = clampPos(target, a.currentDur)
	return fmt.Sprintf("⏩ %s / %s", formatSeconds(a.currentPos), formatSeconds(a.currentDur)), nil
}

// seekRelativeKey seeks by delta seconds and returns a toast command.
func (a *App) seekRelativeKey(delta float64) tea.Cmd {
	if a.facade.State().Current == nil {
		return nil
	}
	if err := a.facade.Seek(delta); err != nil {
		return a.toast.Show(err.Error(), true)
	}
	a.currentPos = clampPos(a.currentPos+delta, a.currentDur)
	return a.toast.Show(fmt.Sprintf("⏩ %s / %s", formatSeconds(a.currentPos), formatSeconds(a.currentDur)), false)
}

// clampPos clamps a seek target into [0, dur] (dur 0 = unknown, only floor).
func clampPos(pos, dur float64) float64 {
	if pos < 0 {
		return 0
	}
	if dur > 0 && pos > dur {
		return dur
	}
	return pos
}

// handleMouse routes wheel scrolling and seekbar clicks/drags.
func (a *App) handleMouse(msg tea.MouseMsg) tea.Cmd {
	// Wheel: scroll the focused list.
	if msg.Button == tea.MouseButtonWheelUp && msg.Action == tea.MouseActionPress {
		a.moveCursorUp()
		return nil
	}
	if msg.Button == tea.MouseButtonWheelDown && msg.Action == tea.MouseActionPress {
		a.moveCursorDown()
		return a.maybeLoadMore()
	}

	// Statusbar seekbar. The status row sits directly above the prompt.
	statusRow := a.height - lipgloss.Height(a.prompt.View()) - 1
	barStart, barWidth, ok := a.statusBar.BarBounds()
	onBar := ok && msg.Y == statusRow && msg.X >= barStart && msg.X < barStart+barWidth

	// Now-playing big bar. Only clickable when that view is showing and no
	// overlay (help/lyrics) is covering it.
	onNpBar := a.currentView == viewNowPlaying && a.helpText == "" && a.lyricsText == "" &&
		a.npBarWidth > 0 && msg.Y == a.npBarRow &&
		msg.X >= a.npBarStart && msg.X < a.npBarStart+a.npBarWidth

	switch {
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && onBar:
		a.seekDragging = true
		a.dragStart, a.dragWidth = barStart, barWidth
		return a.seekToColumn(msg.X, barStart, barWidth)
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && onNpBar:
		a.seekDragging = true
		a.dragStart, a.dragWidth = a.npBarStart, a.npBarWidth
		return a.seekToColumn(msg.X, a.npBarStart, a.npBarWidth)
	case msg.Action == tea.MouseActionMotion && a.seekDragging:
		if time.Since(a.lastDragSeek) >= 250*time.Millisecond {
			return a.seekToColumn(msg.X, a.dragStart, a.dragWidth)
		}
	case msg.Action == tea.MouseActionRelease && a.seekDragging:
		a.seekDragging = false
		return a.seekToColumn(msg.X, a.dragStart, a.dragWidth)
	}
	return nil
}

// seekToColumn seeks to the position that column x represents on a bar
// spanning [barStart, barStart+barWidth).
func (a *App) seekToColumn(x, barStart, barWidth int) tea.Cmd {
	if barWidth <= 0 || a.facade.State().Current == nil {
		return nil
	}
	col := x - barStart
	if col < 0 {
		col = 0
	}
	if col >= barWidth {
		col = barWidth - 1
	}
	pct := float64(col) / float64(barWidth-1) * 100
	if err := a.facade.SeekPercent(pct); err != nil {
		return a.toast.Show(err.Error(), true)
	}
	a.lastDragSeek = time.Now()
	a.currentPos = clampPos(pct/100*a.currentDur, a.currentDur)
	a.statusBar.SetPosition(a.currentPos, a.currentDur)
	return a.toast.Show(fmt.Sprintf("⏩ %s / %s", formatSeconds(a.currentPos), formatSeconds(a.currentDur)), false)
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
	case viewPlaylists:
		if a.playlistCursor > 0 {
			a.playlistCursor--
		}
	case viewPlaylistDetail:
		if a.plDetailCursor > 0 {
			a.plDetailCursor--
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
	case viewPlaylists:
		if a.playlistCursor < len(a.playlists)-1 {
			a.playlistCursor++
		}
	case viewPlaylistDetail:
		if a.plDetailCursor < len(a.currentPlaylist.Tracks)-1 {
			a.plDetailCursor++
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

func (a *App) doSearchMore(query string, fetchTotal int) tea.Cmd {
	return func() tea.Msg {
		results, err := a.facade.SearchMore(context.Background(), query, fetchTotal)
		return SearchMoreMsg{Results: results, Query: query, Err: err}
	}
}

// maybeLoadMore triggers a background refetch when the cursor nears the end
// of the loaded results. Returns nil when no fetch is needed.
func (a *App) maybeLoadMore() tea.Cmd {
	if a.currentView != viewSearch || a.searchQuery == "" {
		return nil
	}
	if a.searchLoadingMore || a.searchExhausted {
		return nil
	}
	if a.searchCursor < len(a.searchResults)-3 {
		return nil
	}
	// After a failure, only retry from the very last row (one retry per gesture).
	if a.searchFetchFailed && a.searchCursor < len(a.searchResults)-1 {
		return nil
	}
	a.searchFetchFailed = false
	max := a.cfg.MaxSearchResults
	if max <= 0 {
		max = 100
	}
	next := core.NextFetchSize(len(a.searchResults), searchBatch, max)
	if next == 0 {
		a.searchExhausted = true
		return nil
	}
	a.searchLoadingMore = true
	return a.doSearchMore(a.searchQuery, next)
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

// doAutoUpdateYtDlp runs the mandatory startup freshness check in the
// background; the TUI renders immediately while this works.
func (a *App) doAutoUpdateYtDlp() tea.Cmd {
	return func() tea.Msg {
		info, updated, err := a.facade.EnsureLatestYtDlp(context.Background())
		return YtDlpAutoUpdateMsg{Info: info, Updated: updated, Err: err}
	}
}

func (a *App) doFetchLyrics() tea.Cmd {
	state := a.facade.State()
	if state.Current == nil {
		return a.toast.Show("Nothing playing", true)
	}
	track := *state.Current
	a.loading = true
	a.loadingText = "Fetching lyrics..."
	return func() tea.Msg {
		text, err := a.facade.FetchLyrics(context.Background(), track.VideoID, track.Title, track.Channel)
		return LyricsLoadedMsg{Lyrics: text, Title: track.Title, Err: err}
	}
}

func (a *App) doDownloadTrack(videoID, title, channel string) tea.Cmd {
	a.loading = true
	a.loadingText = "Downloading..."
	outputDir := config.DownloadDir(a.cfg)
	return func() tea.Msg {
		filePath, err := a.facade.DownloadTrack(context.Background(), videoID, title, channel, outputDir)
		return DownloadCompleteMsg{Title: title, FilePath: filePath, Err: err}
	}
}

func (a *App) doLoadDownloads() tea.Cmd {
	return func() tea.Msg {
		downloads, err := a.facade.ListDownloads(context.Background(), 50)
		return DownloadsLoadedMsg{Downloads: downloads, Err: err}
	}
}

func (a *App) doLoadPlaylists() tea.Cmd {
	return func() tea.Msg {
		playlists, err := a.facade.ListPlaylists(context.Background())
		return PlaylistsLoadedMsg{Playlists: playlists, Err: err}
	}
}

func (a *App) doLoadPlaylistDetail(id int) tea.Cmd {
	return func() tea.Msg {
		p, err := a.facade.GetPlaylist(context.Background(), id)
		return PlaylistDetailMsg{Playlist: p, Err: err}
	}
}

func (a *App) doCreatePlaylist(name string) tea.Cmd {
	return func() tea.Msg {
		_, err := a.facade.CreatePlaylist(context.Background(), name)
		if err != nil {
			return ToastMsg{Text: "Create failed: " + err.Error(), IsErr: true}
		}
		// Reload list
		playlists, _ := a.facade.ListPlaylists(context.Background())
		return PlaylistsLoadedMsg{Playlists: playlists}
	}
}

func (a *App) doPlayPlaylist(id int) tea.Cmd {
	return func() tea.Msg {
		err := a.facade.PlayPlaylist(context.Background(), id)
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

func (a *App) doDeletePlaylist(id int) tea.Cmd {
	return func() tea.Msg {
		err := a.facade.DeletePlaylist(context.Background(), id)
		if err != nil {
			return ToastMsg{Text: "Delete failed: " + err.Error(), IsErr: true}
		}
		playlists, _ := a.facade.ListPlaylists(context.Background())
		return PlaylistsLoadedMsg{Playlists: playlists}
	}
}

func (a *App) doAddToPlaylist(playlistID int) tea.Cmd {
	// Add currently playing track to playlist
	state := a.facade.State()
	if state.Current == nil {
		return a.toast.Show("Nothing playing to add", true)
	}
	track := *state.Current
	return func() tea.Msg {
		err := a.facade.AddToPlaylist(context.Background(), playlistID, track)
		if err != nil {
			return ToastMsg{Text: "Add failed: " + err.Error(), IsErr: true}
		}
		return ToastMsg{Text: "Added to playlist"}
	}
}

func (a *App) doLoadHistory() tea.Cmd {
	return func() tea.Msg {
		entries, err := a.facade.LoadHistory(context.Background(), 50, 0)
		return HistoryLoadedMsg{Entries: entries, Err: err}
	}
}

// focusTickMsg drives the /focus overlay's fake animation forward, once per
// focusTick() interval, for as long as a.focusActive stays true.
type focusTickMsg struct{}

// focusTick schedules the next /focus overlay animation frame.
func focusTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return focusTickMsg{}
	})
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
