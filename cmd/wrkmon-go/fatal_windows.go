//go:build windows

package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// reportFatal prints the error to stderr (for terminal launches) and also
// shows a MessageBox (so double-click launches don't silently flash-close).
func reportFatal(title, detail string) {
	fmt.Fprintf(os.Stderr, "%s\n%s\n", title, detail)

	// MessageBoxW — native Win32, no extra deps beyond x/sys/windows.
	titlePtr, _ := windows.UTF16PtrFromString(title)
	detailPtr, _ := windows.UTF16PtrFromString(detail)
	const MB_OK = 0x00000000
	const MB_ICONERROR = 0x00000010
	windows.MessageBox(0, detailPtr, titlePtr, MB_OK|MB_ICONERROR)

	os.Exit(1)
}
