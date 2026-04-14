package lyrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Fetcher retrieves lyrics from lrclib.net.
type Fetcher struct {
	client *http.Client
}

// NewFetcher creates a lyrics fetcher.
func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type lrclibResponse struct {
	PlainLyrics  string `json:"plainLyrics"`
	SyncedLyrics string `json:"syncedLyrics"`
	TrackName    string `json:"trackName"`
	ArtistName   string `json:"artistName"`
}

// Fetch searches lrclib.net for lyrics matching the track title and artist.
// Returns plain lyrics text, or error if not found.
func (f *Fetcher) Fetch(ctx context.Context, title, artist string) (string, error) {
	// Clean title — remove common suffixes like "(Official Video)", "[Audio]", etc.
	cleaned := cleanTitle(title)

	q := url.Values{}
	q.Set("track_name", cleaned)
	if artist != "" {
		q.Set("artist_name", artist)
	}

	reqURL := "https://lrclib.net/api/search?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "wrkmon-go/2.0 (https://github.com/Umar-Khan-Yousafzai/wrkmon-go)")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("lyrics fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("lyrics API returned %d", resp.StatusCode)
	}

	var results []lrclibResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return "", fmt.Errorf("decode lyrics: %w", err)
	}

	if len(results) == 0 {
		return "", fmt.Errorf("no lyrics found for %q", title)
	}

	// Prefer plain lyrics, fall back to synced
	for _, r := range results {
		if r.PlainLyrics != "" {
			return r.PlainLyrics, nil
		}
	}
	for _, r := range results {
		if r.SyncedLyrics != "" {
			// Strip timestamp tags from synced lyrics
			return stripSyncTags(r.SyncedLyrics), nil
		}
	}

	return "", fmt.Errorf("no lyrics text found for %q", title)
}

func cleanTitle(title string) string {
	// Remove common YouTube suffixes
	for _, suffix := range []string{
		"(Official Video)", "(Official Music Video)", "(Official Audio)",
		"(Audio)", "(Lyric Video)", "(Lyrics)", "(Official Lyric Video)",
		"[Official Video]", "[Official Music Video]", "[Official Audio]",
		"[Audio]", "[Lyric Video]", "[Lyrics]",
		"(Visualizer)", "[Visualizer]",
		"| Official Video", "| Official Audio",
	} {
		title = strings.ReplaceAll(title, suffix, "")
	}
	// Also try case-insensitive
	lower := strings.ToLower(title)
	for _, suffix := range []string{"official video", "official music video", "official audio", "audio", "lyrics", "lyric video"} {
		if idx := strings.Index(lower, suffix); idx > 0 {
			// Check if preceded by ( or [
			if idx > 0 && (title[idx-1] == '(' || title[idx-1] == '[') {
				// Find closing bracket
				end := strings.IndexAny(title[idx:], ")]")
				if end > 0 {
					title = title[:idx-1] + title[idx+end+1:]
					lower = strings.ToLower(title)
				}
			}
		}
	}
	return strings.TrimSpace(title)
}

func stripSyncTags(synced string) string {
	var lines []string
	for _, line := range strings.Split(synced, "\n") {
		// Remove [mm:ss.xx] timestamps
		if idx := strings.Index(line, "]"); idx >= 0 && idx < 12 {
			line = line[idx+1:]
		}
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
