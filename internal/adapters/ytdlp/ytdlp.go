package ytdlp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/ports"
)

// Compile-time interface check.
var _ ports.Searcher = (*Client)(nil)

// Client wraps yt-dlp subprocess calls.
type Client struct {
	mu         sync.RWMutex
	binPath    string // path to yt-dlp binary
	bundled    bool   // whether the binary is wrkmon-owned (managed or bundled) and can self-update
	managedDir string // where the auto-updater installs the managed copy
}

// NewClient creates a yt-dlp client using the locator precedence rule.
// managedDir is where the auto-updater installs a wrkmon-owned copy.
func NewClient(configPath, managedDir string) (*Client, error) {
	result, err := Locate(configPath, managedDir)
	if err != nil {
		return nil, err
	}
	return &Client{binPath: result.Path, bundled: result.Bundled, managedDir: managedDir}, nil
}

// bin returns the current binary path (thread-safe).
func (c *Client) bin() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.binPath
}

// selfUpdatable reports whether the current binary is wrkmon-owned (thread-safe).
func (c *Client) selfUpdatable() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bundled
}

// Relocate atomically switches the client to a different yt-dlp binary.
// In-flight commands finish on the old path; subsequent calls use the new one.
func (c *Client) Relocate(path string, selfUpdatable bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.binPath = path
	c.bundled = selfUpdatable
}

// BinPath returns the resolved yt-dlp binary path.
func (c *Client) BinPath() string { return c.bin() }

// IsBundled reports whether the active binary is wrkmon-owned (managed or bundled).
func (c *Client) IsBundled() bool { return c.selfUpdatable() }

// Update runs yt-dlp -U to self-update the binary.
// Returns the output from yt-dlp.
func (c *Client) Update(ctx context.Context) (string, error) {
	if !c.selfUpdatable() {
		return "System yt-dlp can't self-update — use your package manager", nil
	}
	cmd := exec.CommandContext(ctx, c.bin(), "-U")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("yt-dlp update failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// searchResult is the JSON shape yt-dlp emits per entry in flat-playlist mode.
type searchResult struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Channel  string  `json:"channel"`
	Uploader string  `json:"uploader"`
	Duration float64 `json:"duration"`
}

// Search queries YouTube via yt-dlp and returns up to limit results.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]core.SearchResult, error) {
	searchQuery := "ytsearch" + strconv.Itoa(limit) + ":" + query

	cmd := exec.CommandContext(ctx, c.bin(),
		searchQuery,
		"--dump-json",
		"--flat-playlist",
		"--no-download",
		"--no-warnings",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("yt-dlp start: %w", err)
	}

	var results []core.SearchResult
	scanner := bufio.NewScanner(stdout)
	// Increase buffer for potentially large JSON lines.
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var sr searchResult
		if err := json.Unmarshal([]byte(line), &sr); err != nil {
			continue // skip malformed lines
		}

		channel := sr.Channel
		if channel == "" {
			channel = sr.Uploader
		}

		results = append(results, core.SearchResult{
			VideoID:  sr.ID,
			Title:    sr.Title,
			Channel:  channel,
			Duration: time.Duration(sr.Duration * float64(time.Second)),
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading yt-dlp output: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		// If context was cancelled, return the context error.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("yt-dlp exited with error: %w", err)
	}

	return results, nil
}

// GetStreamURL extracts the best-audio stream URL for a YouTube video.
func (c *Client) GetStreamURL(ctx context.Context, videoID string) (string, error) {
	url := "https://www.youtube.com/watch?v=" + videoID

	cmd := exec.CommandContext(ctx, c.bin(),
		"-f", "bestaudio",
		"--get-url",
		"--no-warnings",
		url,
	)

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("yt-dlp get-url: %w", err)
	}

	streamURL := strings.TrimSpace(string(output))
	if streamURL == "" {
		return "", fmt.Errorf("yt-dlp returned empty stream URL for %s", videoID)
	}

	return streamURL, nil
}

// Download downloads audio for a video to the given directory.
// Returns the output file path.
func (c *Client) Download(ctx context.Context, videoID string, outputDir string) (string, error) {
	url := "https://www.youtube.com/watch?v=" + videoID

	// Use output template to get predictable filename
	outputTmpl := outputDir + "/%(title)s.%(ext)s"

	cmd := exec.CommandContext(ctx, c.bin(),
		"-f", "bestaudio",
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"-o", outputTmpl,
		"--no-warnings",
		"--print", "after_move:filepath",
		url,
	)

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("yt-dlp download: %w", err)
	}

	filePath := strings.TrimSpace(string(output))
	if filePath == "" {
		return "", fmt.Errorf("yt-dlp returned empty file path")
	}
	return filePath, nil
}
