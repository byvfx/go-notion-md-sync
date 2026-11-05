package tui

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// captureMu ensures only one operation redirects process output at a time.
var captureMu sync.Mutex

// captureDrainTimeout bounds how long captureOutput waits for the pipe
// reader after restoring the original output.
const captureDrainTimeout = 2 * time.Second

// captureOutput runs fn while output written to stdout and stderr is
// redirected into a pipe, so sync engine messages do not corrupt the screen
// Bubble Tea is drawing. Every non-empty line written during fn is passed to
// onLine (which may be nil). The original outputs are restored when fn
// returns, including when it panics.
//
// Redirected:
//   - the os.Stdout and os.Stderr variables (fmt.Print*, fmt.Fprint(os.Stderr, ...));
//   - the standard library logger;
//   - on Unix-like systems, file descriptor 1 itself, which catches writers
//     that hold their own reference to the original stdout file (such as
//     the util logger). The TUI renders to stderr, so it is unaffected.
//
// If the pipe cannot be created fn still runs, uncaptured, and the error is
// returned.
func captureOutput(onLine func(string), fn func()) error {
	captureMu.Lock()
	defer captureMu.Unlock()

	r, w, err := os.Pipe()
	if err != nil {
		fn()
		return err
	}

	oldStdout, oldStderr := os.Stdout, os.Stderr
	oldLogOutput := log.Writer()
	restoreFd := redirectStdoutFd(w)

	os.Stdout = w
	os.Stderr = w
	log.SetOutput(w)

	done := make(chan struct{})
	go func() {
		defer close(done)
		readLines(r, onLine)
	}()

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		log.SetOutput(oldLogOutput)
		restoreFd()
		_ = w.Close()

		select {
		case <-done:
		case <-time.After(captureDrainTimeout):
			// Some write end is still open (e.g. a leaked goroutine kept a
			// duplicate); stop reading rather than hang the TUI.
		}
		_ = r.Close()
	}()

	fn()
	return nil
}

// readLines reads r until EOF, passing each non-empty line (split on \n or
// \r, so carriage-return progress output is handled) to onLine.
func readLines(r io.Reader, onLine func(string)) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	scanner.Split(scanLinesOrCR)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && onLine != nil {
			onLine(line)
		}
	}
	// Keep draining after a scanner error (e.g. an overlong line) so
	// writers never block on a full pipe.
	_, _ = io.Copy(io.Discard, r)
}

// scanLinesOrCR is a bufio.SplitFunc that splits on \n or \r.
func scanLinesOrCR(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}
