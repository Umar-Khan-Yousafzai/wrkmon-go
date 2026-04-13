package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds application configuration.
type Config struct {
	Theme          string `toml:"theme"`
	Volume         int    `toml:"volume"`
	YtDlpPath      string `toml:"ytdlp_path"`
	MpvPath        string `toml:"mpv_path"`
	ResultsPerPage int    `toml:"results_per_page"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Theme:          "opencode-mono",
		Volume:         50,
		ResultsPerPage: 10,
	}
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
