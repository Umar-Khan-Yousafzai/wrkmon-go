//go:build !windows

package main

import (
	"fmt"
	"os"
)

// reportFatal prints a multi-line error to stderr and exits.
func reportFatal(title, detail string) {
	fmt.Fprintf(os.Stderr, "%s\n%s\n", title, detail)
	os.Exit(1)
}
