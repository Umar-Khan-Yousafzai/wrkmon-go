package commands

import (
	"fmt"
	"strconv"
	"strings"
)

// SeekKind classifies a parsed /seek argument.
type SeekKind int

const (
	SeekAbsolute SeekKind = iota // seconds from track start
	SeekPct                      // percent of duration, 0–100
	SeekRelative                 // seconds from current position (signed)
)

// SeekSpec is a parsed /seek argument.
type SeekSpec struct {
	Kind  SeekKind
	Value float64
}

// ParseSeek parses a /seek argument. Accepted forms:
//
//	1:23  01:02:03   absolute mm:ss or h:mm:ss
//	83  83.5          absolute seconds
//	50%               percent of duration (0–100)
//	+30  -30          relative seconds
func ParseSeek(arg string) (SeekSpec, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return SeekSpec{}, fmt.Errorf("empty seek target")
	}

	// Relative: leading sign.
	if arg[0] == '+' || arg[0] == '-' {
		v, err := strconv.ParseFloat(arg, 64)
		if err != nil {
			return SeekSpec{}, fmt.Errorf("bad relative offset %q", arg)
		}
		return SeekSpec{SeekRelative, v}, nil
	}

	// Percent: trailing %.
	if strings.HasSuffix(arg, "%") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(arg, "%"), 64)
		if err != nil || v < 0 || v > 100 {
			return SeekSpec{}, fmt.Errorf("percent must be 0–100")
		}
		return SeekSpec{SeekPct, v}, nil
	}

	// Timestamp: contains a colon.
	if strings.Contains(arg, ":") {
		parts := strings.Split(arg, ":")
		if len(parts) < 2 || len(parts) > 3 {
			return SeekSpec{}, fmt.Errorf("timestamp must be mm:ss or h:mm:ss")
		}
		total := 0
		for i, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil || n < 0 {
				return SeekSpec{}, fmt.Errorf("bad timestamp %q", arg)
			}
			// minutes/seconds fields (all but the first) must be < 60
			if i > 0 && n > 59 {
				return SeekSpec{}, fmt.Errorf("bad timestamp %q", arg)
			}
			total = total*60 + n
		}
		return SeekSpec{SeekAbsolute, float64(total)}, nil
	}

	// Plain seconds.
	v, err := strconv.ParseFloat(arg, 64)
	if err != nil || v < 0 {
		return SeekSpec{}, fmt.Errorf("bad seek target %q", arg)
	}
	return SeekSpec{SeekAbsolute, v}, nil
}
