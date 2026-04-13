package main

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/adapters/mpv"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/adapters/store"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/adapters/ytdlp"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/config"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui"
	_ "github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/layouts/single"
)

var version = "dev"

func main() {
	cfg := config.Load()

	// Initialize adapters.
	searcher, err := ytdlp.NewClient(cfg.YtDlpPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yt-dlp not found: %v\nInstall yt-dlp: pip install yt-dlp\n", err)
		os.Exit(1)
	}

	// Check mpv is available.
	mpvBin := "mpv"
	if cfg.MpvPath != "" {
		mpvBin = cfg.MpvPath
	}
	if _, err := exec.LookPath(mpvBin); err != nil {
		fmt.Fprintf(os.Stderr, "mpv not found: %v\nInstall mpv:\n  Ubuntu/Debian: sudo apt install mpv\n  macOS: brew install mpv\n  Windows: winget install mpv\n", err)
		os.Exit(1)
	}

	player := mpv.New()

	dbPath := config.DBPath()
	storage, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "database error: %v\n", err)
		os.Exit(1)
	}
	defer storage.Close()

	// Create facade and app.
	facade := tui.NewFacade(searcher, player, storage)
	defer facade.Close()

	app := tui.NewApp(facade, cfg)

	// Ensure data directory exists.
	os.MkdirAll(config.DataDir(), 0o755)

	// Run the TUI.
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
