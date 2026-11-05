//go:build darwin || freebsd || netbsd || openbsd || dragonfly

package tui

import (
	"os"
	"syscall"
)

// redirectStdoutFd points file descriptor 1 at w and returns a function that
// restores the original descriptor. On failure it leaves fd 1 untouched and
// returns a no-op.
func redirectStdoutFd(w *os.File) func() {
	saved, err := syscall.Dup(1)
	if err != nil {
		return func() {}
	}
	if err := syscall.Dup2(int(w.Fd()), 1); err != nil {
		_ = syscall.Close(saved)
		return func() {}
	}
	return func() {
		_ = syscall.Dup2(saved, 1)
		_ = syscall.Close(saved)
	}
}
