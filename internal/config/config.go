package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
)

// WindowConfig controls the `wrkmon-go window` launcher.
type WindowConfig struct {
	Terminal  string   `toml:"terminal"`   // "auto" or a supported terminal name
	ExtraArgs []string `toml:"extra_args"` // appended to the terminal's argv
}

// Config holds application configuration.
type Config struct {
	Theme            string       `toml:"theme"`
	Volume           int          `toml:"volume"`
	YtDlpPath        string       `toml:"ytdlp_path"`
	MpvPath          string       `toml:"mpv_path"`
	ResultsPerPage   int          `toml:"results_per_page"`
	DownloadDir      string       `toml:"download_dir"`
	Mouse            bool         `toml:"mouse"`
	MaxSearchResults int          `toml:"max_search_results"`
	Window           WindowConfig `toml:"window"`
	AutoUpdateYtDlp  bool         `toml:"auto_update_ytdlp"`
	EQPreset         string       `toml:"eq_preset"`
	EQGains          []float64    `toml:"eq_gains"`
	EQEnabled        bool         `toml:"eq_enabled"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Theme:            "opencode-mono",
		Volume:           50,
		ResultsPerPage:   10,
		Mouse:            true,
		MaxSearchResults: 100,
		Window:           WindowConfig{Terminal: "auto"},
		AutoUpdateYtDlp:  true,
		EQPreset:         "flat",
		EQGains:          nil,
		EQEnabled:        false,
	}
}

// EQState reconstructs a core.EQState from the persisted config fields.
// Persisted gains that don't have exactly 18 entries (e.g. a hand-edited or
// corrupted config file) are treated as flat rather than trusted verbatim.
func (c Config) EQState() core.EQState {
	e := core.EQState{Preset: c.EQPreset, Enabled: c.EQEnabled}
	if len(c.EQGains) != 18 {
		e.Preset = "flat"
		return e
	}
	copy(e.Gains[:], c.EQGains)
	return e
}

// configDir returns ~/.config/wrkmon-go/
func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "wrkmon-go")
}

// configPath returns the full config file path.
func configPath() string {
	return filepath.Join(configDir(), "config.toml")
}

// Load reads config from disk, falling back to defaults.
func Load() Config {
	cfg := DefaultConfig()
	path := configPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return DefaultConfig()
	}
	return cfg
}

// Save writes config to disk.
func Save(cfg Config) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(configPath())
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

// DataDir returns ~/.local/share/wrkmon-go/
func DataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "wrkmon-go")
}

// DBPath returns the full database path.
func DBPath() string {
	return filepath.Join(DataDir(), "wrkmon.db")
}

// DownloadDir returns the download directory, defaulting to ~/Music/wrkmon-go/.
func DownloadDir(cfg Config) string {
	if cfg.DownloadDir != "" {
		return cfg.DownloadDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Music", "wrkmon-go")
}
