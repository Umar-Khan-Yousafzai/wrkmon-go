package mpv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// LocateResult describes which mpv binary was found and how.
type LocateResult struct {
	Path    string
	Bundled bool   // true if found next to the wrkmon-go binary
	Source  string // human-readable description
}

// Locate finds the mpv binary using a 4-tier precedence:
//  1. Config override (explicit path or bare name)
//  2. Bundled next to the wrkmon-go binary (or in a `mpv` subdirectory)
//  3. System PATH
//  4. Error
func Locate(configPath string) (LocateResult, error) {
	if configPath != "" {
		if isPath(configPath) {
			if _, err := os.Stat(configPath); err != nil {
				return LocateResult{}, fmt.Errorf("configured mpv path not found: %s", configPath)
			}
			return LocateResult{Path: configPath, Source: "config"}, nil
		}
		p, err := exec.LookPath(configPath)
		if err != nil {
			return LocateResult{}, fmt.Errorf("configured mpv %q not found on PATH", configPath)
		}
		return LocateResult{Path: p, Source: "config (PATH)"}, nil
	}

	bundledName := "mpv"
	if runtime.GOOS == "windows" {
		bundledName = "mpv.exe"
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates := []string{
			filepath.Join(exeDir, bundledName),
			filepath.Join(exeDir, "mpv", bundledName),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return LocateResult{Path: c, Bundled: true, Source: "bundled"}, nil
			}
		}
	}

	if p, err := exec.LookPath("mpv"); err == nil {
		return LocateResult{Path: p, Source: "system PATH"}, nil
	}

	return LocateResult{}, fmt.Errorf("mpv not found.\nInstall mpv or place the mpv binary next to wrkmon-go")
}

func isPath(s string) bool {
	return strings.ContainsAny(s, "/\\") || filepath.IsAbs(s)
}
