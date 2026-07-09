package ytdlp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// fakeYtDlp is a script that answers --version like a real yt-dlp build.
const fakeYtDlp = "#!/bin/sh\necho 2026.07.01\n"

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("unix shell-script fake binary")
	}
}

func TestEnsureLatestMigratesSystemBinary(t *testing.T) {
	skipOnWindows(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fakeYtDlp))
	}))
	defer srv.Close()
	old := releaseBaseURL
	releaseBaseURL = srv.URL + "/"
	defer func() { releaseBaseURL = old }()

	dir := t.TempDir()
	c := &Client{binPath: "/usr/bin/yt-dlp", bundled: false, managedDir: dir}

	msg, updated, err := c.EnsureLatest(context.Background())
	if err != nil {
		t.Fatalf("EnsureLatest: %v", err)
	}
	if !updated {
		t.Error("migration must report updated=true")
	}
	want := filepath.Join(dir, "yt-dlp")
	if c.BinPath() != want {
		t.Errorf("BinPath = %s, want %s (hot-swap)", c.BinPath(), want)
	}
	if !c.IsBundled() {
		t.Error("managed copy must be self-updatable after swap")
	}
	info, err := os.Stat(want)
	if err != nil {
		t.Fatalf("managed binary missing: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %o, want 755", info.Mode().Perm())
	}
	if msg == "" {
		t.Error("expected a human-readable message")
	}
	if _, err := os.Stat(filepath.Join(dir, ".yt-dlp.tmp")); !os.IsNotExist(err) {
		t.Error("temp file must not remain after success")
	}
}

func TestEnsureLatestDownloadFailureKeepsOldBinary(t *testing.T) {
	skipOnWindows(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	old := releaseBaseURL
	releaseBaseURL = srv.URL + "/"
	defer func() { releaseBaseURL = old }()

	dir := t.TempDir()
	c := &Client{binPath: "/usr/bin/yt-dlp", bundled: false, managedDir: dir}
	_, updated, err := c.EnsureLatest(context.Background())
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if updated {
		t.Error("updated must be false on failure")
	}
	if c.BinPath() != "/usr/bin/yt-dlp" {
		t.Errorf("binPath changed on failure: %s", c.BinPath())
	}
}

func TestEnsureLatestBadBinaryRejected(t *testing.T) {
	skipOnWindows(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("#!/bin/sh\necho not-a-version\n"))
	}))
	defer srv.Close()
	old := releaseBaseURL
	releaseBaseURL = srv.URL + "/"
	defer func() { releaseBaseURL = old }()

	dir := t.TempDir()
	c := &Client{binPath: "/usr/bin/yt-dlp", bundled: false, managedDir: dir}
	_, updated, err := c.EnsureLatest(context.Background())
	if err == nil || updated {
		t.Fatal("expected verify failure")
	}
	if c.BinPath() != "/usr/bin/yt-dlp" {
		t.Error("binPath must be unchanged after failed verify")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "yt-dlp")); !os.IsNotExist(statErr) {
		t.Error("bad binary must not be installed")
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".yt-dlp.tmp")); !os.IsNotExist(statErr) {
		t.Error("temp file must be cleaned up")
	}
}

// TestEnsureLatestConcurrentCallsDoNotRace reproduces the fixed-tmp-path
// race: two goroutines both call EnsureLatest against a system binary at
// the same time. Before the updateMu serialization fix, the loser would
// either fail to rename its temp file (winner already renamed it away) or,
// worse, could truncate the shared inode mid-verification. With the fix,
// the second caller blocks until the first finishes migrating, then takes
// the fast self-update (-U) path against the now-managed binary.
func TestEnsureLatestConcurrentCallsDoNotRace(t *testing.T) {
	skipOnWindows(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fakeYtDlp))
	}))
	defer srv.Close()
	old := releaseBaseURL
	releaseBaseURL = srv.URL + "/"
	defer func() { releaseBaseURL = old }()

	dir := t.TempDir()
	c := &Client{binPath: "/usr/bin/yt-dlp", bundled: false, managedDir: dir}

	var wg sync.WaitGroup
	results := make([]struct {
		msg     string
		updated bool
		err     error
	}, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			msg, updated, err := c.EnsureLatest(context.Background())
			results[i].msg = msg
			results[i].updated = updated
			results[i].err = err
		}(i)
	}
	wg.Wait()

	anyUpdated := false
	for i, r := range results {
		if r.err != nil {
			t.Errorf("caller %d: EnsureLatest returned error: %v (msg=%q)", i, r.err, r.msg)
		}
		if r.updated {
			anyUpdated = true
		}
	}
	if !anyUpdated {
		t.Error("expected at least one caller to report updated=true")
	}

	want := filepath.Join(dir, "yt-dlp")
	if c.BinPath() != want {
		t.Errorf("BinPath = %s, want %s (hot-swap)", c.BinPath(), want)
	}
	if !c.IsBundled() {
		t.Error("managed copy must be self-updatable after swap")
	}

	info, err := os.Stat(want)
	if err != nil {
		t.Fatalf("managed binary missing: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %o, want 755", info.Mode().Perm())
	}

	out, err := exec.CommandContext(context.Background(), want, "--version").Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "2026.07.01" {
		t.Errorf("--version = %q, want %q (content must not be truncated/corrupted)", got, "2026.07.01")
	}

	if _, err := os.Stat(filepath.Join(dir, ".yt-dlp.tmp")); !os.IsNotExist(err) {
		t.Error("temp file must not remain after both callers finish")
	}
}

func TestEnsureLatestSkipsConfigPinned(t *testing.T) {
	dir := t.TempDir()
	c := &Client{binPath: "/custom/yt-dlp", bundled: false, configPinned: true, managedDir: dir}

	msg, updated, err := c.EnsureLatest(context.Background())
	if err != nil {
		t.Fatalf("EnsureLatest: %v", err)
	}
	if updated {
		t.Error("config-pinned binary must never report updated=true")
	}
	if !strings.Contains(msg, "pinned") {
		t.Errorf("msg = %q, want it to mention \"pinned\"", msg)
	}
	if c.BinPath() != "/custom/yt-dlp" {
		t.Errorf("BinPath = %s, want unchanged /custom/yt-dlp", c.BinPath())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("managedDir must stay empty, found %v", entries)
	}
}

func TestAssetName(t *testing.T) {
	// Pins the per-OS asset table regardless of host OS.
	cases := []struct{ goos, goarch, want string }{
		{"windows", "amd64", "yt-dlp.exe"},
		{"darwin", "arm64", "yt-dlp_macos"},
		{"darwin", "amd64", "yt-dlp_macos"},
		{"linux", "arm64", "yt-dlp_linux_aarch64"},
		{"linux", "amd64", "yt-dlp_linux"},
		{"freebsd", "amd64", "yt-dlp_linux"},
	}
	for _, c := range cases {
		if got := assetNameFor(c.goos, c.goarch); got != c.want {
			t.Errorf("assetNameFor(%s,%s) = %s, want %s", c.goos, c.goarch, got, c.want)
		}
	}
}
