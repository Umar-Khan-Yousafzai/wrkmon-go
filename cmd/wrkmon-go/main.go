package main

import (
	"fmt"
	"os"

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

	searcher, err := ytdlp.NewClient(cfg.YtDlpPath)
	if err != nil {
		reportFatal("wrkmon-go: yt-dlp not found",
			fmt.Sprintf("%v\n\nInstall wrkmon-go via the official installer to provision yt-dlp automatically, or place yt-dlp next to the wrkmon-go binary.", err))
	}

	player, err := mpv.New(cfg.MpvPath)
	if err != nil {
		reportFatal("wrkmon-go: mpv not found",
			fmt.Sprintf("%v\n\nInstall wrkmon-go via the official installer to provision mpv automatically, or install mpv from your system package manager.", err))
	}

	dbPath := config.DBPath()
	storage, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		reportFatal("wrkmon-go: database error", err.Error())
	}
	defer storage.Close()

	facade := tui.NewFacade(searcher, player, storage)
	defer facade.Close()

	app := tui.NewApp(facade, cfg)

	os.MkdirAll(config.DataDir(), 0o755)

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
