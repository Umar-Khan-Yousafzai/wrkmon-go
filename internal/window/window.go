// Package window launches wrkmon-go inside a dedicated terminal-emulator
// window in "app mode" (own window class, no tabs), so the TUI behaves
// like a desktop application. The window class is always "wrkmon-go" and
// must match StartupWMClass in assets/wrkmon-go.desktop.
package window

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// termSpec describes one supported terminal emulator.
type termSpec struct {
	name string
	args func(self string, extra []string) []string
}

// specs returns the priority-ordered launch table for this OS.
func specs() []termSpec {
	return specsFor(runtime.GOOS)
}

// specsFor returns the priority-ordered launch table for the given GOOS.
// Factored out of specs() so tests can exercise the Windows table on any
// host without depending on runtime.GOOS.
func specsFor(goos string) []termSpec {
	switch goos {
	case "windows":
		return []termSpec{
			{"wt", func(self string, extra []string) []string {
				return append(append([]string{"--title", "wrkmon"}, extra...), self)
			}},
		}
	default: // linux, darwin, bsd — X11/Wayland terminals
		return []termSpec{
			{"kitty", func(self string, extra []string) []string {
				return append(append([]string{"--class=wrkmon-go", "--title=wrkmon"}, extra...), "-e", self)
			}},
			{"alacritty", func(self string, extra []string) []string {
				return append(append([]string{"--class", "wrkmon-go", "--title", "wrkmon"}, extra...), "-e", self)
			}},
			{"wezterm", func(self string, extra []string) []string {
				return append(append([]string{"start", "--class", "wrkmon-go"}, extra...), "--", self)
			}},
			{"ghostty", func(self string, extra []string) []string {
				return append(append([]string{"--class=wrkmon-go"}, extra...), "-e", self)
			}},
			{"foot", func(self string, extra []string) []string {
				return append(append([]string{"--app-id=wrkmon-go", "--title=wrkmon"}, extra...), self)
			}},
			{"cosmic-term", func(self string, extra []string) []string {
				return append(append([]string{}, extra...), "--", self)
			}},
			{"gnome-terminal", func(self string, extra []string) []string {
				return append(append([]string{"--title=wrkmon"}, extra...), "--", self)
			}},
			{"konsole", func(self string, extra []string) []string {
				return append(append([]string{"--separate"}, extra...), "-e", self)
			}},
			{"xterm", func(self string, extra []string) []string {
				return append(append([]string{"-class", "wrkmon-go", "-T", "wrkmon"}, extra...), "-e", self)
			}},
		}
	}
}

// Resolve picks the terminal binary and argv. override is "auto" (or "")
// to probe the table in order, or a terminal name from the table.
// lookPath is exec.LookPath, injectable for tests.
func Resolve(self, override string, extra []string, lookPath func(string) (string, error)) (string, []string, error) {
	table := specs()

	if override != "" && override != "auto" {
		for _, s := range table {
			if s.name == override {
				bin, err := lookPath(s.name)
				if err != nil {
					return "", nil, fmt.Errorf("configured terminal %q is not installed", override)
				}
				return bin, s.args(self, extra), nil
			}
		}
		return "", nil, fmt.Errorf("unknown terminal %q; supported: %s", override, names(table))
	}

	for _, s := range table {
		if bin, err := lookPath(s.name); err == nil {
			return bin, s.args(self, extra), nil
		}
	}
	return "", nil, fmt.Errorf(
		"no supported terminal found (looked for %s); set [window] terminal in ~/.config/wrkmon-go/config.toml",
		names(table))
}

func names(table []termSpec) string {
	out := make([]string, len(table))
	for i, s := range table {
		out[i] = s.name
	}
	return strings.Join(out, ", ")
}

// shouldFallback reports whether Launch may fall back to an OS-level
// terminal (conhost/Terminal.app) when Resolve fails to find a configured
// terminal. Fallbacks only apply to the "probe the table" cases (no
// override, or the explicit "auto" override) — if the user explicitly named
// a terminal in [window] terminal and it's missing or misspelled, Resolve's
// error must be returned as-is instead of silently launching something else.
func shouldFallback(override string) bool {
	return override == "" || override == "auto"
}

// Launch opens the app window and returns once the terminal is spawned.
func Launch(override string, extra []string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	bin, args, err := Resolve(self, override, extra, exec.LookPath)
	if err != nil {
		if !shouldFallback(override) {
			return err
		}
		// OS-level fallbacks that aren't PATH-probed terminals.
		switch runtime.GOOS {
		case "windows":
			// The empty quoted "" is the START command's window-title
			// argument; without it, an unquoted first arg (e.g. "wrkmon")
			// is parsed by START as the program to run instead of a title.
			return exec.Command("cmd", "/c", "start", "", self).Start()
		case "darwin":
			return exec.Command("osascript", "-e",
				fmt.Sprintf(`tell application "Terminal" to do script %q`, self)).Start()
		}
		return err
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launching %s: %w", bin, err)
	}
	return cmd.Process.Release()
}
