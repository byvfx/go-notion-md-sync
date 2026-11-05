//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package tui

import "os"

// redirectStdoutFd is a no-op on platforms without dup2; only the os.Stdout
// and os.Stderr variables and the standard logger are redirected there.
func redirectStdoutFd(_ *os.File) func() {
	return func() {}
}
