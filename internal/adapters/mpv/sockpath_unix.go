//go:build !windows

package mpv

import (
	"fmt"
	"os"
	"path/filepath"
)

func socketPath(pid int) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("wrkmon-mpv-%d.sock", pid))
}

func removeSocketFile(path string) {
	os.Remove(path)
}
