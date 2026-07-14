package core

import (
	"strings"
	"testing"
)

func TestEQPresetKnown(t *testing.T) {
	e, ok := EQPreset("bass")
	if !ok || e.Preset != "bass" || !e.Enabled {
		t.Fatalf("bass preset broken: %+v ok=%v", e, ok)
	}
	if e.Gains[0] <= 0 {
		t.Fatalf("bass must boost low bands, got %v", e.Gains[0])
	}
}

func TestEQPresetUnknown(t *testing.T) {
	if _, ok := EQPreset("nope"); ok {
		t.Fatal("unknown preset must return ok=false")
	}
}

func TestSetBandValidation(t *testing.T) {
	var e EQState
	for _, c := range []struct {
		n       int
		db      float64
		wantErr bool
	}{
		{0, 3, true}, {19, 3, true}, {1, 12.1, true}, {1, -12.1, true}, {1, -12, false}, {18, 12, false},
	} {
		if err := e.SetBand(c.n, c.db); (err != nil) != c.wantErr {
			t.Errorf("SetBand(%d,%v) err=%v want err=%v", c.n, c.db, err, c.wantErr)
		}
	}
	if e.Preset != "custom" {
		t.Fatalf("valid SetBand must set preset=custom, got %q", e.Preset)
	}
}

func TestFilterString(t *testing.T) {
	var e EQState
	if got := e.FilterString(); got != "" {
		t.Fatalf("disabled/flat => empty, got %q", got)
	}
	e.Enabled = true
	if got := e.FilterString(); got != "" {
		t.Fatalf("all-zero gains => empty, got %q", got)
	}
	e.Gains[0] = 6 // +6 dB => multiplier 10^(6/20) ≈ 1.995
	got := e.FilterString()
	if !strings.HasPrefix(got, "lavfi=[superequalizer=") || !strings.Contains(got, "1b=2.0") {
		t.Fatalf("bad filter string: %q", got)
	}
}
