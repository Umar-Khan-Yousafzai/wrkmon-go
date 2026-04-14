//go:build windows

package mpv

import "fmt"

func socketPath(pid int) string {
	// mpv IPC on Windows uses named pipes; bare names create a regular file.
	return fmt.Sprintf(`\\.\pipe\wrkmon-mpv-%d`, pid)
}

func removeSocketFile(path string) {
	// Named pipes aren't removable via the file API; closing the pipe handle
	// (done in stop()) is sufficient.
	_ = path
}
