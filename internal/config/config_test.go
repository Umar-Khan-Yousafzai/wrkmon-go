package config

import (
	"os"
	"path/filepath"
	"testing"
)

func setTempHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp) // windows
	return tmp
}

func TestDefaults(t *testing.T) {
	setTempHome(t)
	cfg := Load() // no file on disk
	if !cfg.Mouse {
		t.Error("Mouse should default to true")
	}
	if cfg.MaxSearchResults != 100 {
		t.Errorf("MaxSearchResults = %d, want 100", cfg.MaxSearchResults)
	}
}

func TestLoadKeepsDefaultsForMissingKeys(t *testing.T) {
	tmp := setTempHome(t)
	dir := filepath.Join(tmp, ".config", "wrkmon-go")
	os.MkdirAll(dir, 0o755)
	// A pre-existing config that predates the new keys.
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("theme = \"github-dark\"\nvolume = 70\n"), 0o644)
	cfg := Load()
	if !cfg.Mouse {
		t.Error("Mouse should stay true when key absent")
	}
	if cfg.MaxSearchResults != 100 {
		t.Errorf("MaxSearchResults = %d, want 100", cfg.MaxSearchResults)
	}
	if cfg.Volume != 70 {
		t.Errorf("Volume = %d, want 70", cfg.Volume)
	}
}

func TestLoadHonorsExplicitValues(t *testing.T) {
	tmp := setTempHome(t)
	dir := filepath.Join(tmp, ".config", "wrkmon-go")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("mouse = false\nmax_search_results = 40\n"), 0o644)
	cfg := Load()
	if cfg.Mouse {
		t.Error("mouse = false should be honored")
	}
	if cfg.MaxSearchResults != 40 {
		t.Errorf("MaxSearchResults = %d, want 40", cfg.MaxSearchResults)
	}
}

func TestWindowDefaults(t *testing.T) {
	setTempHome(t)
	cfg := Load()
	if cfg.Window.Terminal != "auto" {
		t.Errorf("Window.Terminal = %q, want auto", cfg.Window.Terminal)
	}
}

func TestAutoUpdateYtDlpDefaultsTrue(t *testing.T) {
	setTempHome(t)
	cfg := Load()
	if !cfg.AutoUpdateYtDlp {
		t.Error("AutoUpdateYtDlp should default to true")
	}
}
