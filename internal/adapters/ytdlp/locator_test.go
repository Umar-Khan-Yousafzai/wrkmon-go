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
