package core

import (
	"fmt"
	"math"
	"strings"
)

// EQState holds the 18-band superequalizer state. Gains are in dB (−12..+12).
type EQState struct {
	Preset  string
	Gains   [18]float64
	Enabled bool
}

var eqPresets = map[string][18]float64{
	"flat":   {},
	"bass":   {6, 5, 4, 3, 2, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	"treble": {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4, 5, 5, 6},
	"vocal":  {-2, -1, 0, 1, 2, 3, 4, 4, 4, 3, 2, 1, 0, 0, -1, -1, -2, -2},
	"rock":   {5, 4, 3, 1, 0, -1, -1, 0, 1, 2, 3, 3, 4, 4, 4, 4, 4, 5},
	"pop":    {-1, 0, 1, 2, 4, 4, 3, 2, 1, 0, 0, 1, 2, 3, 3, 2, 1, 0},
}

var EQPresetNames = []string{"bass", "flat", "pop", "rock", "treble", "vocal"}

func EQPreset(name string) (EQState, bool) {
	g, ok := eqPresets[name]
	if !ok {
		return EQState{}, false
	}
	return EQState{Preset: name, Gains: g, Enabled: true}, true
}

func (e *EQState) SetBand(n int, db float64) error {
	if n < 1 || n > 18 {
		return fmt.Errorf("band must be 1-18, got %d", n)
	}
	if db < -12 || db > 12 {
		return fmt.Errorf("gain must be -12..+12 dB, got %v", db)
	}
	e.Gains[n-1] = db
	e.Preset = "custom"
	e.Enabled = true
	return nil
}

// FilterString renders the mpv af value. Empty string means "no filter".
func (e EQState) FilterString() string {
	if !e.Enabled {
		return ""
	}
	allZero := true
	for _, g := range e.Gains {
		if g != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return ""
	}
	parts := make([]string, 18)
	for i, db := range e.Gains {
		mult := math.Pow(10, db/20) // dB → linear multiplier; superequalizer range 0..20, 1=unity
		parts[i] = fmt.Sprintf("%db=%.1f", i+1, mult)
	}
	return "lavfi=[superequalizer=" + strings.Join(parts, ":") + "]"
}
