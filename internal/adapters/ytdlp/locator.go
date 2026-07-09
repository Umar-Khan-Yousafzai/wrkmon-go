package ytdlp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// LocateResult describes which yt-dlp binary was found and how.
type LocateResult struct {
	Path    string
	Bundled bool   // true if the binary is wrkmon-owned (managed or bundled)
	Source  string // human-readable description
}

// Locate finds the yt-dlp binary using the 5-tier precedence rule:
//  1. Config override (explicit path or bare name)
//  2. Managed copy in managedDir (installed by wrkmon's auto-updater)
//  3. Bundled next to the wrkmon binary
//  4. System PATH
//  5. Error
func Locate(configPath, managedDir string) (LocateResult, error) {
	// 1. Config override
	if configPath != "" {
		if isPath(configPath) {
			// Looks like a file path — use directly
			if _, err := os.Stat(configPath); err != nil {
				return LocateResult{}, fmt.Errorf("configured yt-dlp path not found: %s", configPath)
			}
			return LocateResult{Path: configPath, Source: "config"}, nil
		}
		// Bare name — look up on PATH
		p, err := exec.LookPath(configPath)
		if err != nil {
			return LocateResult{}, fmt.Errorf("configured yt-dlp %q not found on PATH", configPath)
		}
		return LocateResult{Path: p, Source: "config (PATH)"}, nil
	}

	// 2. Managed copy (wrkmon-owned, self-updatable)
	if managedDir != "" {
		managedName := "yt-dlp"
		if runtime.GOOS == "windows" {
			managedName = "yt-dlp.exe"
		}
		managedPath := filepath.Join(managedDir, managedName)
		if _, err := os.Stat(managedPath); err == nil {
			return LocateResult{Path: managedPath, Bundled: true, Source: "managed"}, nil
		}
	}

	// 3. Bundled next to wrkmon binary
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		bundledName := "yt-dlp"
		if runtime.GOOS == "windows" {
			bundledName = "yt-dlp.exe"
		}
		bundledPath := filepath.Join(exeDir, bundledName)
		if _, err := os.Stat(bundledPath); err == nil {
			return LocateResult{Path: bundledPath, Bundled: true, Source: "bundled"}, nil
		}
	}

	// 4. System PATH
	p, err := exec.LookPath("yt-dlp")
	if err == nil {
		return LocateResult{Path: p, Source: "system PATH"}, nil
	}

	// 5. Not found
	return LocateResult{}, fmt.Errorf("yt-dlp not found.\nInstall: pip install yt-dlp\n  Or place yt-dlp binary next to wrkmon-go")
}

func isPath(s string) bool {
	return strings.ContainsAny(s, "/\\") || filepath.IsAbs(s)
}
