package ytdlp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
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
