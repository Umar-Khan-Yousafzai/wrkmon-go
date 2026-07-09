# yt-dlp Startup Auto-Update Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every wrkmon-go start checks yt-dlp freshness in the background; system-PATH copies are migrated once to a wrkmon-managed copy that self-updates thereafter, hot-swapped without restart.

**Architecture:** A new "managed" locator tier (`<DataDir>/bin/yt-dlp`) outranks bundled/system; `Client` gains a mutex-guarded binPath with `Relocate`; a new `EnsureLatest` entry point either runs `-U` (self-updatable binaries) or downloads the official standalone build and swaps to it; the TUI fires it from `Init` and toasts results.

**Tech Stack:** Go stdlib only (`net/http`, `httptest`, `sync`), Bubble Tea messages, existing 4-tier locator.

**Spec:** `docs/superpowers/specs/2026-07-09-ytdlp-auto-update-design.md`

## Global Constraints

- Zero CGO; stdlib only — no new dependencies
- Managed dir is exactly `filepath.Join(config.DataDir(), "bin")` = `~/.local/share/wrkmon-go/bin/`; managed binary name `yt-dlp` (`yt-dlp.exe` on Windows)
- Locator precedence: config → managed → bundled → system PATH → error
- Download URL: `https://github.com/yt-dlp/yt-dlp/releases/latest/download/<asset>`; assets: windows `yt-dlp.exe`, darwin `yt-dlp_macos`, linux/arm64 `yt-dlp_linux_aarch64`, linux/other `yt-dlp_linux`, any other GOOS `yt-dlp_linux`
- Download safety: temp file `.yt-dlp.tmp` in the managed dir → chmod 0755 (non-Windows) → verify `--version` output matches `^\d{4}\.\d{2}\.\d{2}` → atomic `os.Rename`; failed verify deletes the temp file; HTTP timeout 5 minutes
- Config key `auto_update_ytdlp`, default `true`
- Startup check must NOT delay TUI launch (background `tea.Cmd` from `Init`)
- Toast rules: silent when already current and no error; `Updated` → info toast; error → warning toast; app keeps working on any failure
- Cross-platform mandatory: `GOOS=windows GOARCH=amd64 go build ./...` and `GOOS=darwin GOARCH=arm64 go build ./...` must pass; unix-only tests skip on Windows via `runtime.GOOS` guard
- `go test ./... -short` green + `gofmt -l .` empty before every commit; conventional commits, NO Co-Authored-By or AI attribution lines

---

### Task 1: Managed locator tier

**Files:**
- Modify: `internal/adapters/ytdlp/locator.go` (Locate signature + new tier)
- Modify: `internal/adapters/ytdlp/ytdlp.go` (NewClient signature, store managedDir)
- Modify: `cmd/wrkmon-go/main.go` (NewClient call site)
- Test: `internal/adapters/ytdlp/locator_test.go` (new)

**Interfaces:**
- Consumes: existing `Locate(configPath string)` 4-tier logic, `config.DataDir()`
- Produces: `Locate(configPath, managedDir string) (LocateResult, error)` with `Source == "managed"`, `Bundled == true` for the managed tier; `NewClient(configPath, managedDir string) (*Client, error)`; `Client` field `managedDir string`. Tasks 2–4 rely on these exact signatures.

- [ ] **Step 1: Write the failing test** — `internal/adapters/ytdlp/locator_test.go`:

```go
package ytdlp

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func managedName() string {
	if runtime.GOOS == "windows" {
		return "yt-dlp.exe"
	}
	return "yt-dlp"
}

func TestLocateManagedTierWins(t *testing.T) {
	dir := t.TempDir()
	managed := filepath.Join(dir, managedName())
	if err := os.WriteFile(managed, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Locate("", dir)
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if res.Path != managed {
		t.Errorf("Path = %s, want managed copy %s", res.Path, managed)
	}
	if !res.Bundled {
		t.Error("managed copy must report Bundled=true (self-updatable)")
	}
	if res.Source != "managed" {
		t.Errorf("Source = %q, want \"managed\"", res.Source)
	}
}

func TestLocateEmptyManagedDirFallsThrough(t *testing.T) {
	dir := t.TempDir() // exists but has no yt-dlp
	res, err := Locate("", dir)
	// Depending on the host, this resolves bundled/system or errors —
	// either way it must NOT resolve to the managed dir.
	if err == nil && filepath.Dir(res.Path) == dir {
		t.Errorf("resolved into empty managed dir: %s", res.Path)
	}
}

func TestLocateConfigStillBeatsManaged(t *testing.T) {
	dir := t.TempDir()
	managed := filepath.Join(dir, managedName())
	os.WriteFile(managed, []byte("#!/bin/sh\n"), 0o755)
	cfgBin := filepath.Join(dir, "custom-ytdlp")
	os.WriteFile(cfgBin, []byte("#!/bin/sh\n"), 0o755)
	res, err := Locate(cfgBin, dir)
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if res.Path != cfgBin {
		t.Errorf("config override lost to managed tier: %s", res.Path)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapters/ytdlp/ -run TestLocate -v`
Expected: compile error — `too many arguments in call to Locate`

- [ ] **Step 3: Implement** — in `internal/adapters/ytdlp/locator.go`:

Change the doc comment and signature, and insert the managed tier between tier 1 and the bundled tier:

```go
// Locate finds the yt-dlp binary using the 5-tier precedence rule:
//  1. Config override (explicit path or bare name)
//  2. Managed copy in managedDir (installed by wrkmon's auto-updater)
//  3. Bundled next to the wrkmon binary
//  4. System PATH
//  5. Error
func Locate(configPath, managedDir string) (LocateResult, error) {
```

Immediately after the config-override block (before the "Bundled next to wrkmon binary" comment), add:

```go
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
```

In `internal/adapters/ytdlp/ytdlp.go`, change `NewClient` and the `Client` struct:

```go
// Client wraps yt-dlp subprocess calls.
type Client struct {
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
```

In `cmd/wrkmon-go/main.go`, update the call site (currently `ytdlp.NewClient(cfg.YtDlpPath)`):

```go
	searcher, err := ytdlp.NewClient(cfg.YtDlpPath, filepath.Join(config.DataDir(), "bin"))
```

Add `"path/filepath"` to main.go's imports.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/adapters/ytdlp/ -run TestLocate -v && go build ./... && go test ./... -short`
Expected: all PASS, build clean

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/ytdlp/locator.go internal/adapters/ytdlp/locator_test.go internal/adapters/ytdlp/ytdlp.go cmd/wrkmon-go/main.go
git commit -m "feat(ytdlp): managed locator tier in DataDir/bin"
```

---

### Task 2: Thread-safe binPath + Relocate

**Files:**
- Modify: `internal/adapters/ytdlp/ytdlp.go` (mutex, accessor, Relocate; switch all command builders to the accessor)
- Test: `internal/adapters/ytdlp/ytdlp_test.go` (append)

**Interfaces:**
- Consumes: `Client` struct from Task 1
- Produces: `(c *Client) Relocate(path string, selfUpdatable bool)`; internal accessor `(c *Client) bin() string`; `BinPath()`/`IsBundled()` now mutex-guarded. Task 3 calls `Relocate` after a successful download.

- [ ] **Step 1: Write the failing test** — append to `internal/adapters/ytdlp/ytdlp_test.go`:

```go
func TestRelocateSwapsBinary(t *testing.T) {
	c := &Client{binPath: "/old/yt-dlp", bundled: false}
	c.Relocate("/new/yt-dlp", true)
	if c.BinPath() != "/new/yt-dlp" {
		t.Errorf("BinPath = %s, want /new/yt-dlp", c.BinPath())
	}
	if !c.IsBundled() {
		t.Error("IsBundled must be true after Relocate(…, true)")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapters/ytdlp/ -run TestRelocate -v`
Expected: compile error — `c.Relocate undefined`

- [ ] **Step 3: Implement** — in `internal/adapters/ytdlp/ytdlp.go`:

Add `"sync"` to imports. Add the mutex as the FIRST struct field and the new methods:

```go
// Client wraps yt-dlp subprocess calls.
type Client struct {
	mu         sync.RWMutex
	binPath    string // path to yt-dlp binary
	bundled    bool   // whether the binary is wrkmon-owned (managed or bundled) and can self-update
	managedDir string // where the auto-updater installs the managed copy
}
```

```go
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
```

Convert the existing accessors and every command builder to go through the guarded reads. `BinPath()`/`IsBundled()` become:

```go
// BinPath returns the resolved yt-dlp binary path.
func (c *Client) BinPath() string { return c.bin() }

// IsBundled reports whether the active binary is wrkmon-owned (managed or bundled).
func (c *Client) IsBundled() bool { return c.selfUpdatable() }
```

Replace `c.binPath` with `c.bin()` in the FOUR command builders — `Update` (`exec.CommandContext(ctx, c.bin(), "-U")`), `Search`, `GetStreamURL`, `Download` — and `!c.bundled` in `Update` with `!c.selfUpdatable()`. After this change, `grep -n "c\.binPath" internal/adapters/ytdlp/ytdlp.go` must only show the struct field, `NewClient`, and the bodies of `bin()`/`Relocate()`.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/adapters/ytdlp/ -short -v && go vet ./internal/adapters/ytdlp/ && go test ./... -short`
Expected: PASS; vet clean (vet's copylocks check also proves no `Client` is copied by value anywhere)

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/ytdlp/ytdlp.go internal/adapters/ytdlp/ytdlp_test.go
git commit -m "feat(ytdlp): thread-safe binPath with Relocate for hot-swapping"
```

---

### Task 3: EnsureLatest — self-update or migrate-and-swap

**Files:**
- Create: `internal/adapters/ytdlp/update.go`
- Test: `internal/adapters/ytdlp/update_test.go` (new)
- Modify: `internal/tui/facade.go` (`UpdateYtDlp` prefers `EnsureLatest`)

**Interfaces:**
- Consumes: `Relocate` (Task 2), `c.managedDir` (Task 1)
- Produces: `(c *Client) EnsureLatest(ctx context.Context) (msg string, updated bool, err error)`; package-level `var releaseBaseURL = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/"` (overridable in tests); `Facade.EnsureLatestYtDlp(ctx) (string, bool, error)`. Task 4 calls the facade method from the TUI.

- [ ] **Step 1: Write the failing test** — `internal/adapters/ytdlp/update_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapters/ytdlp/ -run 'TestEnsure|TestAssetName' -v`
Expected: compile error — `undefined: releaseBaseURL`

- [ ] **Step 3: Implement** — `internal/adapters/ytdlp/update.go`:

```go
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
```

In `internal/tui/facade.go`, replace the body of `UpdateYtDlp` so the manual `/update-ytdlp` command gains migration ability:

```go
// UpdateYtDlp brings yt-dlp up to date (self-update or managed migration).
func (f *Facade) UpdateYtDlp(ctx context.Context) (string, error) {
	msg, _, err := f.EnsureLatestYtDlp(ctx)
	return msg, err
}

// EnsureLatestYtDlp exposes the searcher's EnsureLatest when supported.
func (f *Facade) EnsureLatestYtDlp(ctx context.Context) (string, bool, error) {
	type ensurer interface {
		EnsureLatest(ctx context.Context) (string, bool, error)
	}
	if e, ok := f.searcher.(ensurer); ok {
		return e.EnsureLatest(ctx)
	}
	return "yt-dlp update not supported with this searcher", false, nil
}
```

(The old `updater` type-assert block in `UpdateYtDlp` is removed.)

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/adapters/ytdlp/ -short -v && go test ./... -short && go vet ./...`
Expected: all PASS (the three Ensure tests + TestAssetName + existing suites)

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/ytdlp/update.go internal/adapters/ytdlp/update_test.go internal/tui/facade.go
git commit -m "feat(ytdlp): EnsureLatest self-update or managed migration with hot-swap"
```

---

### Task 4: Config gate + startup wiring + toasts

**Files:**
- Modify: `internal/config/config.go` (+ append test to `internal/config/config_test.go`)
- Modify: `internal/tui/messages.go`
- Modify: `internal/tui/app.go` (`Init`, new `doAutoUpdateYtDlp`, message handler)

**Interfaces:**
- Consumes: `Facade.EnsureLatestYtDlp(ctx) (string, bool, error)` (Task 3)
- Produces: `Config.AutoUpdateYtDlp bool` (toml `auto_update_ytdlp`, default true); `YtDlpAutoUpdateMsg{Info string; Updated bool; Err error}`.

- [ ] **Step 1: Write the failing config test** — append to `internal/config/config_test.go`:

```go
func TestAutoUpdateYtDlpDefaultsTrue(t *testing.T) {
	setTempHome(t)
	cfg := Load()
	if !cfg.AutoUpdateYtDlp {
		t.Error("AutoUpdateYtDlp should default to true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestAutoUpdate -v`
Expected: compile error — `cfg.AutoUpdateYtDlp undefined`

- [ ] **Step 3: Implement config** — in `internal/config/config.go` add to the `Config` struct:

```go
	AutoUpdateYtDlp bool `toml:"auto_update_ytdlp"`
```

and to `DefaultConfig()`:

```go
		AutoUpdateYtDlp: true,
```

Run: `go test ./internal/config/ -v` — Expected: PASS

- [ ] **Step 4: Message type** — append to `internal/tui/messages.go`:

```go
// YtDlpAutoUpdateMsg carries the result of the startup yt-dlp check.
type YtDlpAutoUpdateMsg struct {
	Info    string
	Updated bool
	Err     error
}
```

- [ ] **Step 5: Startup command + handler** — in `internal/tui/app.go`:

Change `Init` (currently returns `textinput.Blink`):

```go
func (a *App) Init() tea.Cmd {
	// Apply saved volume
	if a.cfg.Volume > 0 {
		_ = a.facade.SetVolume(a.cfg.Volume)
	}
	cmds := []tea.Cmd{textinput.Blink}
	if a.cfg.AutoUpdateYtDlp {
		cmds = append(cmds, a.doAutoUpdateYtDlp())
	}
	return tea.Batch(cmds...)
}
```

Add next to `doUpdateYtDlp` (~line 1590):

```go
// doAutoUpdateYtDlp runs the mandatory startup freshness check in the
// background; the TUI renders immediately while this works.
func (a *App) doAutoUpdateYtDlp() tea.Cmd {
	return func() tea.Msg {
		info, updated, err := a.facade.EnsureLatestYtDlp(context.Background())
		return YtDlpAutoUpdateMsg{Info: info, Updated: updated, Err: err}
	}
}
```

Add a handler case next to the existing `YtDlpUpdateMsg` case (~line 717):

```go
	case YtDlpAutoUpdateMsg:
		if msg.Err != nil {
			cmds = append(cmds, a.toast.Show("yt-dlp auto-update: "+msg.Err.Error(), true))
		} else if msg.Updated {
			cmds = append(cmds, a.toast.Show(msg.Info, false))
		}
		// silent when already current
```

- [ ] **Step 6: Verify + cross-compile**

Run: `go build ./... && go vet ./... && gofmt -l . && go test ./... -short && GOOS=windows GOARCH=amd64 go build ./... && GOOS=darwin GOARCH=arm64 go build ./...`
Expected: gofmt prints nothing; everything green.

Manual: `go run ./cmd/wrkmon-go` on this machine (stale system yt-dlp 2024.04) → TUI opens instantly; within ~seconds a toast `yt-dlp 20XX.XX.XX installed (managed copy)`; `ls ~/.local/share/wrkmon-go/bin/yt-dlp` exists; a YouTube search then works; second launch shows no toast (silent `-U` current path).

- [ ] **Step 7: Commit**

```bash
git add internal/config/ internal/tui/messages.go internal/tui/app.go
git commit -m "feat(tui): mandatory yt-dlp freshness check on every startup"
```
