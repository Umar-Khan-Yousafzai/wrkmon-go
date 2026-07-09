package ytdlp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// releaseBaseURL is where official standalone yt-dlp builds live.
// Package-level var so tests can point it at a local server.
var releaseBaseURL = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/"

// versionRe matches yt-dlp's CalVer output, e.g. "2026.07.01".
var versionRe = regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}`)

// assetNameFor returns the official release asset for an OS/arch pair.
func assetNameFor(goos, goarch string) string {
	switch goos {
	case "windows":
		return "yt-dlp.exe"
	case "darwin":
		return "yt-dlp_macos"
	case "linux":
		if goarch == "arm64" {
			return "yt-dlp_linux_aarch64"
		}
		return "yt-dlp_linux"
	default:
		return "yt-dlp_linux"
	}
}

// EnsureLatest brings the active yt-dlp up to date.
//   - Self-updatable binary (managed/bundled): runs `yt-dlp -U`.
//   - System binary: downloads the official standalone build into the
//     managed dir and hot-swaps to it (one-time migration).
//
// Returns a human-readable message and whether anything changed.
func (c *Client) EnsureLatest(ctx context.Context) (string, bool, error) {
	if c.selfUpdatable() {
		return c.selfUpdate(ctx)
	}
	return c.migrate(ctx)
}

// selfUpdate runs -U on a wrkmon-owned binary.
func (c *Client) selfUpdate(ctx context.Context) (string, bool, error) {
	out, err := exec.CommandContext(ctx, c.bin(), "-U").CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return text, false, fmt.Errorf("yt-dlp -U failed: %w", err)
	}
	updated := strings.Contains(text, "Updating to") || strings.Contains(text, "Updated yt-dlp")
	if !updated {
		return text, false, nil // already current — caller stays silent
	}
	v, verr := binaryVersion(ctx, c.bin())
	if verr != nil {
		v = "latest"
	}
	return "yt-dlp updated → " + v, true, nil
}

// migrate downloads the official standalone build into the managed dir,
// verifies it, and hot-swaps the client to it.
func (c *Client) migrate(ctx context.Context) (string, bool, error) {
	if c.managedDir == "" {
		return "", false, fmt.Errorf("no managed dir configured")
	}
	if err := os.MkdirAll(c.managedDir, 0o755); err != nil {
		return "", false, fmt.Errorf("managed dir %s: %w", c.managedDir, err)
	}

	name := "yt-dlp"
	if runtime.GOOS == "windows" {
		name = "yt-dlp.exe"
	}
	tmp := filepath.Join(c.managedDir, ".yt-dlp.tmp")
	final := filepath.Join(c.managedDir, name)

	url := releaseBaseURL + assetNameFor(runtime.GOOS, runtime.GOARCH)
	if err := downloadTo(ctx, url, tmp); err != nil {
		os.Remove(tmp)
		return "", false, err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmp, 0o755); err != nil {
			os.Remove(tmp)
			return "", false, fmt.Errorf("chmod: %w", err)
		}
	}
	v, err := binaryVersion(ctx, tmp)
	if err != nil {
		os.Remove(tmp)
		return "", false, fmt.Errorf("downloaded yt-dlp failed verification: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return "", false, fmt.Errorf("install: %w", err)
	}
	c.Relocate(final, true)
	return "yt-dlp " + v + " installed (managed copy)", true, nil
}

// downloadTo streams url into dest (following redirects).
func downloadTo(ctx context.Context, url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download yt-dlp: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download yt-dlp: HTTP %d from %s", resp.StatusCode, url)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return fmt.Errorf("write %s: %w", dest, err)
	}
	return f.Close()
}

// binaryVersion runs --version and validates the output shape.
func binaryVersion(ctx context.Context, bin string) (string, error) {
	out, err := exec.CommandContext(ctx, bin, "--version").Output()
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(out))
	if !versionRe.MatchString(v) {
		return "", fmt.Errorf("unexpected --version output %q", v)
	}
	return v, nil
}
