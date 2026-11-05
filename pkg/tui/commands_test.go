package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/byvfx/go-notion-md-sync/pkg/staging"
	tea "github.com/charmbracelet/bubbletea"
)

// fakeEngine records calls and can fail or panic for specific paths.
type fakeEngine struct {
	mu        gosync.Mutex
	pushed    []string
	pulled    map[string]string // path -> page ID
	syncAll   []string
	failPaths map[string]bool
	panicOn   string
	onPush    func(path string)
	block     chan struct{}
}

func newFakeEngine() *fakeEngine {
	return &fakeEngine{pulled: make(map[string]string), failPaths: make(map[string]bool)}
}

func (f *fakeEngine) SyncFileToNotion(ctx context.Context, filePath string) error {
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.panicOn != "" && strings.HasSuffix(filePath, f.panicOn) {
		panic("boom")
	}
	if f.failPaths[filepath.Base(filePath)] {
		return errors.New("api error")
	}
	if f.onPush != nil {
		f.onPush(filePath)
	}
	f.pushed = append(f.pushed, filePath)
	return nil
}

func (f *fakeEngine) SyncNotionToFile(ctx context.Context, pageID, filePath string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failPaths[filepath.Base(filePath)] {
		return errors.New("api error")
	}
	f.pulled[filePath] = pageID
	return nil
}

func (f *fakeEngine) SyncAll(ctx context.Context, direction string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.syncAll = append(f.syncAll, direction)
	return nil
}

func (f *fakeEngine) SyncSpecificFile(ctx context.Context, filename, direction string) error {
	return nil
}

func testConfig(root string) *config.Config {
	cfg := &config.Config{}
	cfg.Notion.Token = "token"
	cfg.Notion.ParentPageID = "parent"
	cfg.Directories.MarkdownRoot = root
	cfg.Directories.ExcludedPatterns = []string{"*.tmp", "drafts/**"}
	return cfg
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

// runOperation runs an operation synchronously and returns its final message.
func runOperation(t *testing.T, ce *CommandExecutor, cmd CommandType, files []string) tea.Msg {
	t.Helper()
	var run operationFunc
	switch cmd {
	case CommandSync:
		run = ce.runSync
	case CommandPull:
		run = ce.runPull
	case CommandPush:
		run = ce.runPush
	default:
		t.Fatalf("unsupported command %s", cmd)
	}
	start, work, err := ce.prepareOperation(cmd, files, run)
	if err != nil {
		t.Fatalf("prepareOperation() error = %v", err)
	}
	if _, ok := start().(CommandStartMsg); !ok {
		t.Fatalf("start message has wrong type")
	}
	return work()
}

func TestExecutor_Operations(t *testing.T) {
	tests := []struct {
		name        string
		cmd         CommandType
		selected    []string
		failFiles   []string
		wantErr     string
		wantMessage string
		wantCount   int
		wantPushed  []string
		wantPulled  map[string]string
		wantSyncAll []string
	}{
		{
			name:        "push selected files only",
			cmd:         CommandPush,
			selected:    []string{"b.md", "a.md"},
			wantMessage: "Pushed 2 file(s)",
			wantCount:   2,
			wantPushed:  []string{"a.md", "b.md"},
		},
		{
			name:        "push selected directory expands to markdown files",
			cmd:         CommandPush,
			selected:    []string{"Parent"},
			wantMessage: "Pushed 2 file(s)",
			wantCount:   2,
			wantPushed:  []string{"Parent/Child/Child.md", "Parent/Parent.md"},
		},
		{
			name:        "push with nothing selected pushes everything",
			cmd:         CommandPush,
			wantMessage: "Pushed all files",
			wantSyncAll: []string{"push"},
			wantCount:   5,
		},
		{
			name:       "push failure is reported as failure",
			cmd:        CommandPush,
			selected:   []string{"a.md", "b.md"},
			failFiles:  []string{"b.md"},
			wantErr:    "pushed 1 of 2 file(s); failed: b.md: api error",
			wantCount:  1,
			wantPushed: []string{"a.md"},
		},
		{
			name:    "sync with nothing selected is an error",
			cmd:     CommandSync,
			wantErr: "select files to sync",
		},
		{
			name:        "sync pushes selected files",
			cmd:         CommandSync,
			selected:    []string{"a.md"},
			wantMessage: "Synced 1 file(s)",
			wantCount:   1,
			wantPushed:  []string{"a.md"},
		},
		{
			name:        "pull selected uses notion_id and skips files without one",
			cmd:         CommandPull,
			selected:    []string{"linked.md", "a.md"},
			wantMessage: "Pulled 1 file(s) from Notion; skipped 1 without notion_id (a.md)",
			wantCount:   1,
			wantPulled:  map[string]string{"linked.md": "page-123"},
		},
		{
			name:     "pull selected without notion_id is an error",
			cmd:      CommandPull,
			selected: []string{"a.md"},
			wantErr:  "nothing pulled; skipped 1 without notion_id (a.md)",
		},
		{
			name:        "pull with nothing selected pulls everything",
			cmd:         CommandPull,
			wantMessage: "Pulled all pages",
			wantSyncAll: []string{"pull"},
		},
		{
			name:      "pull failure is reported as failure",
			cmd:       CommandPull,
			selected:  []string{"linked.md"},
			failFiles: []string{"linked.md"},
			wantErr:   "pulled 0 of 1 file(s); failed: linked.md: api error",
		},
		{
			name:     "missing selected file is an error",
			cmd:      CommandPush,
			selected: []string{"missing.md"},
			wantErr:  "cannot access missing.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "a.md"), "# A\n")
			writeTestFile(t, filepath.Join(root, "b.md"), "# B\n")
			writeTestFile(t, filepath.Join(root, "linked.md"), "---\nnotion_id: page-123\n---\n# Linked\n")
			writeTestFile(t, filepath.Join(root, "Parent", "Parent.md"), "# Parent\n")
			writeTestFile(t, filepath.Join(root, "Parent", "Child", "Child.md"), "# Child\n")
			writeTestFile(t, filepath.Join(root, "drafts", "excluded.md"), "# Draft\n")

			engine := newFakeEngine()
			for _, f := range tt.failFiles {
				engine.failPaths[f] = true
			}
			ce := newCommandExecutorWithEngine(testConfig(root), engine)
			defer ce.Close()

			msg := runOperation(t, ce, tt.cmd, tt.selected)

			switch m := msg.(type) {
			case CommandCompleteMsg:
				if tt.wantErr != "" {
					t.Fatalf("expected failure %q, got completion %q", tt.wantErr, m.Message)
				}
				if !strings.Contains(m.Message, tt.wantMessage) {
					t.Errorf("message = %q, want it to contain %q", m.Message, tt.wantMessage)
				}
				if m.Count != tt.wantCount {
					t.Errorf("count = %d, want %d", m.Count, tt.wantCount)
				}
			case CommandErrorMsg:
				if tt.wantErr == "" {
					t.Fatalf("unexpected failure: %v", m.Error)
				}
				if !strings.Contains(m.Error.Error(), tt.wantErr) {
					t.Errorf("error = %q, want it to contain %q", m.Error, tt.wantErr)
				}
				if m.Count != tt.wantCount {
					t.Errorf("count = %d, want %d", m.Count, tt.wantCount)
				}
			default:
				t.Fatalf("unexpected message %T", msg)
			}

			var gotPushed []string
			for _, p := range engine.pushed {
				rel, err := filepath.Rel(root, p)
				if err != nil {
					t.Fatalf("Rel() error = %v", err)
				}
				gotPushed = append(gotPushed, filepath.ToSlash(rel))
			}
			if fmt.Sprint(gotPushed) != fmt.Sprint(tt.wantPushed) {
				t.Errorf("pushed = %v, want %v", gotPushed, tt.wantPushed)
			}

			gotPulled := make(map[string]string)
			for p, id := range engine.pulled {
				rel, err := filepath.Rel(root, p)
				if err != nil {
					t.Fatalf("Rel() error = %v", err)
				}
				gotPulled[filepath.ToSlash(rel)] = id
			}
			if len(tt.wantPulled) != len(gotPulled) {
				t.Errorf("pulled = %v, want %v", gotPulled, tt.wantPulled)
			}
			for p, id := range tt.wantPulled {
				if gotPulled[p] != id {
					t.Errorf("pulled[%s] = %q, want %q", p, gotPulled[p], id)
				}
			}

			if fmt.Sprint(engine.syncAll) != fmt.Sprint(tt.wantSyncAll) {
				t.Errorf("SyncAll calls = %v, want %v", engine.syncAll, tt.wantSyncAll)
			}

			if ce.IsRunning() {
				t.Error("executor still marked running after operation")
			}
		})
	}
}

// TestExecutor_ProgressIsRaceFree is a regression test for unsynchronised
// progress fields; run with -race.
func TestExecutor_ProgressIsRaceFree(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 20; i++ {
		writeTestFile(t, filepath.Join(root, fmt.Sprintf("f%02d.md", i)), "# F\n")
	}
	engine := newFakeEngine()
	ce := newCommandExecutorWithEngine(testConfig(root), engine)
	defer ce.Close()

	var files []string
	for i := 0; i < 20; i++ {
		files = append(files, fmt.Sprintf("f%02d.md", i))
	}

	start, work, err := ce.prepareOperation(CommandPush, files, ce.runPush)
	if err != nil {
		t.Fatalf("prepareOperation() error = %v", err)
	}
	_ = start()

	done := make(chan tea.Msg, 1)
	go func() { done <- work() }()

	var final tea.Msg
	for final == nil {
		select {
		case final = <-done:
		default:
			_ = ce.Progress()
			_ = ce.IsRunning()
		}
	}
	if _, ok := final.(CommandCompleteMsg); !ok {
		t.Fatalf("expected completion, got %T", final)
	}
}

func TestExecutor_SingleOperationAtATime(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a.md"), "# A\n")
	engine := newFakeEngine()
	ce := newCommandExecutorWithEngine(testConfig(root), engine)
	defer ce.Close()

	_, work, err := ce.prepareOperation(CommandPush, []string{"a.md"}, ce.runPush)
	if err != nil {
		t.Fatalf("prepareOperation() error = %v", err)
	}
	if _, _, err := ce.prepareOperation(CommandPull, nil, ce.runPull); err == nil {
		t.Error("expected second operation to be rejected while the first is running")
	}
	_ = work()

	ce.setProgress("stale message")
	if _, _, err := ce.prepareOperation(CommandPull, nil, ce.runPull); err != nil {
		t.Fatalf("expected operation to start after the first finished: %v", err)
	}
	if msg := ce.Progress().Message; msg != "" {
		t.Errorf("progress message not reset at start of operation: %q", msg)
	}
}

func TestExecutor_PanicIsReportedAsFailure(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a.md"), "# A\n")
	engine := newFakeEngine()
	engine.panicOn = "a.md"
	ce := newCommandExecutorWithEngine(testConfig(root), engine)
	defer ce.Close()

	stdout, stderr := os.Stdout, os.Stderr
	msg := runOperation(t, ce, CommandPush, []string{"a.md"})
	if _, ok := msg.(CommandErrorMsg); !ok {
		t.Fatalf("expected CommandErrorMsg, got %T", msg)
	}
	if ce.IsRunning() {
		t.Error("executor still running after panic")
	}
	if os.Stdout != stdout || os.Stderr != stderr {
		t.Error("stdout/stderr not restored after panic")
	}
}

// TestExecutor_CapturesEngineOutput is a regression test: engine output
// printed during an operation must not reach the terminal.
func TestExecutor_CapturesEngineOutput(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a.md"), "# A\n")
	engine := newFakeEngine()
	var lines []string
	var linesMu gosync.Mutex
	engine.onPush = func(path string) {
		fmt.Printf("Creating page %s\n", filepath.Base(path))
		_, _ = fmt.Fprintln(os.Stderr, "warning from engine")
	}
	ce := newCommandExecutorWithEngine(testConfig(root), engine)
	defer ce.Close()

	stdout, stderr := os.Stdout, os.Stderr

	err := captureOutput(func(line string) {
		linesMu.Lock()
		defer linesMu.Unlock()
		lines = append(lines, line)
	}, func() {
		if _, err := ce.runPush(context.Background(), []string{"a.md"}); err != nil {
			t.Errorf("runPush() error = %v", err)
		}
	})
	if err != nil {
		t.Fatalf("captureOutput() error = %v", err)
	}

	if os.Stdout != stdout || os.Stderr != stderr {
		t.Fatal("stdout/stderr not restored")
	}
	linesMu.Lock()
	defer linesMu.Unlock()
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Creating page a.md") || !strings.Contains(joined, "warning from engine") {
		t.Errorf("captured output = %q", joined)
	}
}

func TestCaptureOutput_RestoresOnPanic(t *testing.T) {
	stdout, stderr := os.Stdout, os.Stderr
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic to propagate")
			}
		}()
		_ = captureOutput(nil, func() {
			fmt.Println("captured before panic")
			panic("boom")
		})
	}()
	if os.Stdout != stdout || os.Stderr != stderr {
		t.Error("stdout/stderr not restored after panic")
	}
}

func TestScanLinesOrCR(t *testing.T) {
	var got []string
	readLines(strings.NewReader("one\rtwo\nthree\r\n\nfour"), func(line string) {
		got = append(got, line)
	})
	want := []string{"one", "two", "three", "four"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("lines = %v, want %v", got, want)
	}
}

// TestExecutor_MarksSyncedInStaging ensures pushed files show as synced in
// the staging area afterwards.
func TestExecutor_MarksSyncedInStaging(t *testing.T) {
	wd := t.TempDir()
	root := filepath.Join(wd, "docs")
	writeTestFile(t, filepath.Join(root, "a.md"), "# A\n")

	area := staging.NewStagingArea(wd)
	if err := area.Initialize(); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	engine := newFakeEngine()
	engine.onPush = func(path string) {
		// Simulate notion_id write-back.
		_ = os.WriteFile(path, []byte("---\nnotion_id: x\n---\n# A\n"), 0644)
	}
	ce := newCommandExecutorWithEngine(testConfig(root), engine)
	ce.staging = area
	defer ce.Close()

	if msg := runOperation(t, ce, CommandPush, []string{"a.md"}); msg == nil {
		t.Fatal("no message")
	}

	status, err := area.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if st, ok := status["docs/a.md"]; ok {
		t.Errorf("expected docs/a.md to be synced, got %v", st)
	}
}

// TestUnifiedView_FailureNotReportedAsCompleted ensures failed operations
// are shown as failed and do not count as synced files.
func TestUnifiedView_FailureNotReportedAsCompleted(t *testing.T) {
	v := NewUnifiedView()
	v.stats.today = 0
	model, _ := v.Update(CommandStartMsg{Command: "push", StartTime: time.Now()})
	v = model.(UnifiedView)
	model, _ = v.Update(CommandErrorMsg{Command: "push", Error: errors.New("api down")})
	v = model.(UnifiedView)

	if len(v.syncOps) != 1 || !strings.HasPrefix(v.syncOps[0].Status, "Failed") {
		t.Errorf("sync ops = %+v, want one failed operation", v.syncOps)
	}
	if v.stats.today != 0 {
		t.Errorf("today = %d, want 0", v.stats.today)
	}

	model, _ = v.Update(CommandStartMsg{Command: "pull", StartTime: time.Now()})
	v = model.(UnifiedView)
	model, _ = v.Update(CommandCompleteMsg{Command: "pull", Message: "Pulled 2 file(s)", Count: 2})
	v = model.(UnifiedView)
	if v.stats.today != 2 {
		t.Errorf("today = %d, want 2", v.stats.today)
	}
}
