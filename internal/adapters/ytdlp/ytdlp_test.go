package ytdlp

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestNewClient_NotFound(t *testing.T) {
	_, err := NewClient("/nonexistent/path/to/yt-dlp-fake-binary")
	if err == nil {
		t.Fatal("expected error for non-existent binary, got nil")
	}
}

func TestNewClient_FindsOnPath(t *testing.T) {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skip("yt-dlp not installed, skipping")
	}

	client, err := NewClient("")
	if err != nil {
		t.Fatalf("NewClient(\"\") failed: %v", err)
	}
	if client.binPath == "" {
		t.Fatal("expected non-empty binPath")
	}
}

func TestSearch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skip("yt-dlp not installed, skipping")
	}

	client, err := NewClient("")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := client.Search(ctx, "never gonna give you up", 3)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least 1 result, got 0")
	}

	for i, r := range results {
		if r.VideoID == "" {
			t.Errorf("result[%d]: empty VideoID", i)
		}
		if r.Title == "" {
			t.Errorf("result[%d]: empty Title", i)
		}
	}

	t.Logf("got %d results, first: %q by %q (%s)",
		len(results), results[0].Title, results[0].Channel, results[0].Duration)
}

func TestGetStreamURL_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skip("yt-dlp not installed, skipping")
	}

	client, err := NewClient("")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	streamURL, err := client.GetStreamURL(ctx, "dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("GetStreamURL failed: %v", err)
	}

	if streamURL == "" {
		t.Fatal("expected non-empty stream URL")
	}

	t.Logf("stream URL length: %d, prefix: %.80s...", len(streamURL), streamURL)
}
