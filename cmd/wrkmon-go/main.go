package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/adapters/mediakeys"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/adapters/mpv"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/adapters/store"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/adapters/ytdlp"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/config"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui"
	_ "github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/layouts/single"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/window"
	tea "github.com/charmbracelet/bubbletea"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v", "version":
			fmt.Printf("wrkmon-go %s\n", version)
			return
		}
	}

	if len(os.Args) > 1 && (os.Args[1] == "window" || os.Args[1] == "--window") {
		cfg := config.Load()
		if err := window.Launch(cfg.Window.Terminal, cfg.Window.ExtraArgs); err != nil {
			fmt.Fprintln(os.Stderr, "wrkmon-go window:", err)
			os.Exit(1)
		}
		return
	}

	cfg := config.Load()

	searcher, err := ytdlp.NewClient(cfg.YtDlpPath, filepath.Join(config.DataDir(), "bin"))
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

	// Only construct a real per-OS media-key adapter when the user hasn't
	// disabled it — avoids opening OS media-session resources (D-Bus, etc)
	// for a feature that's turned off. When disabled, remote stays the nil
	// core.MediaRemote; App.Init is nil-safe and starts no listener.
	var remote core.MediaRemote
	if cfg.MediaKeys {
		remote = mediakeys.New("wrkmon-go")
	}
	if remote != nil {
		defer remote.Close()
	}

	tui.AppVersion = version
	app := tui.NewApp(facade, cfg, remote)

	os.MkdirAll(config.DataDir(), 0o755)

	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if cfg.Mouse {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	p := tea.NewProgram(app, opts...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
