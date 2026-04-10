//go:build !windows

package mpv

import "net"

func dialSocket(path string) (net.Conn, error) {
	return net.Dial("unix", path)
}
