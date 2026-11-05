//go:build linux

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
	if err := syscall.Dup3(int(w.Fd()), 1, 0); err != nil {
		_ = syscall.Close(saved)
		return func() {}
	}
	return func() {
		_ = syscall.Dup3(saved, 1, 0)
		_ = syscall.Close(saved)
	}
}
