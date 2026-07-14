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

func TestEQDefaults(t *testing.T) {
	setTempHome(t)
	cfg := Load()
	if cfg.EQPreset != "flat" {
		t.Errorf("EQPreset default = %q, want flat", cfg.EQPreset)
	}
	if cfg.EQEnabled {
		t.Error("EQEnabled should default to false")
	}
	if cfg.EQGains != nil {
		t.Errorf("EQGains default = %v, want nil", cfg.EQGains)
	}
}

func TestEQRoundTrip(t *testing.T) {
	setTempHome(t)
	cfg := DefaultConfig()
	cfg.EQPreset = "bass"
	cfg.EQEnabled = true
	gains := make([]float64, 18)
	for i := range gains {
		gains[i] = float64(i) - 5
	}
	cfg.EQGains = gains

	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded := Load()
	if loaded.EQPreset != "bass" {
		t.Errorf("EQPreset = %q, want bass", loaded.EQPreset)
	}
	if !loaded.EQEnabled {
		t.Error("EQEnabled should round-trip to true")
	}
	if len(loaded.EQGains) != 18 {
		t.Fatalf("EQGains len = %d, want 18", len(loaded.EQGains))
	}
	for i, want := range gains {
		if loaded.EQGains[i] != want {
			t.Errorf("EQGains[%d] = %v, want %v", i, loaded.EQGains[i], want)
		}
	}

	state := loaded.EQState()
	if state.Preset != "bass" || !state.Enabled {
		t.Fatalf("EQState() = %+v, want preset=bass enabled=true", state)
	}
	for i, want := range gains {
		if state.Gains[i] != want {
			t.Errorf("EQState().Gains[%d] = %v, want %v", i, state.Gains[i], want)
		}
	}
}

func TestEQStateTreatsWrongLengthGainsAsFlat(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EQPreset = "bass"
	cfg.EQEnabled = true
	cfg.EQGains = []float64{1, 2, 3} // malformed: not 18 entries

	state := cfg.EQState()
	if state.Preset != "flat" {
		t.Errorf("Preset = %q, want flat when gains are malformed", state.Preset)
	}
	for i, g := range state.Gains {
		if g != 0 {
			t.Errorf("Gains[%d] = %v, want 0 when gains are malformed", i, g)
		}
	}
}
