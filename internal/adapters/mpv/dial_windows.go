//go:build windows

package mpv

import (
	"fmt"
	"net"
)

func dialSocket(path string) (net.Conn, error) {
	// Windows named pipe support not yet implemented.
	// mpv on Windows uses \\.\pipe\<name> for IPC.
	// Would need github.com/Microsoft/go-winio or similar library.
	_ = path
	return nil, fmt.Errorf("windows named pipe IPC not yet implemented")
}
