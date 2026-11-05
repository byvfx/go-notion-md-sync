package tui

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	gosync "sync"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/byvfx/go-notion-md-sync/pkg/markdown"
	"github.com/byvfx/go-notion-md-sync/pkg/staging"
	"github.com/byvfx/go-notion-md-sync/pkg/sync"
	"github.com/byvfx/go-notion-md-sync/pkg/util"
	tea "github.com/charmbracelet/bubbletea"
)

// CommandType represents the type of command being executed
type CommandType string

const (
	CommandSync   CommandType = "sync"
	CommandPull   CommandType = "pull"
	CommandPush   CommandType = "push"
	CommandWatch  CommandType = "watch"
	CommandStatus CommandType = "status"
	CommandInit   CommandType = "init"
)

// CommandExecutor handles executing sync commands in the background
type CommandExecutor struct {
	config     *config.Config
	syncEngine sync.Engine
	staging    *staging.StagingArea
	ctx        context.Context
	cancelFunc context.CancelFunc

	// mu guards the progress fields below, which are written by the
	// operation goroutine and read by the UI goroutine.
	mu                 gosync.Mutex
	currentOperation   string
	operationStartTime time.Time
	lastProgressMsg    string
	isRunning          bool
}

// ProgressSnapshot is a consistent copy of the executor's progress state.
type ProgressSnapshot struct {
	Operation string
	StartTime time.Time
	Message   string
	Running   bool
}

// NewCommandExecutor creates a new command executor
func NewCommandExecutor(cfg *config.Config) (*CommandExecutor, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}
	return newCommandExecutorWithEngine(cfg, sync.NewEngine(cfg)), nil
}

func newCommandExecutorWithEngine(cfg *config.Config, engine sync.Engine) *CommandExecutor {
	ctx, cancel := context.WithCancel(context.Background())
	ce := &CommandExecutor{
		config:     cfg,
		syncEngine: engine,
		ctx:        ctx,
		cancelFunc: cancel,
	}
	// The CLI's staging commands use the working directory as the staging
	// root; use the same one so status stays consistent.
	if wd, err := os.Getwd(); err == nil {
		ce.staging = staging.NewStagingArea(wd)
	}
	return ce
}

// Close cleans up the executor resources
func (ce *CommandExecutor) Close() {
	if ce.cancelFunc != nil {
		ce.cancelFunc()
	}
}

// Progress returns a snapshot of the current operation's progress.
func (ce *CommandExecutor) Progress() ProgressSnapshot {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	return ProgressSnapshot{
		Operation: ce.currentOperation,
		StartTime: ce.operationStartTime,
		Message:   ce.lastProgressMsg,
		Running:   ce.isRunning,
	}
}

// IsRunning reports whether an operation is in progress.
func (ce *CommandExecutor) IsRunning() bool {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	return ce.isRunning
}

// begin marks an operation as started. It returns false (and the name of
// the running operation) if another operation is already in progress.
func (ce *CommandExecutor) begin(operation string) (time.Time, string, bool) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	if ce.isRunning {
		return time.Time{}, ce.currentOperation, false
	}
	ce.isRunning = true
	ce.currentOperation = operation
	ce.operationStartTime = time.Now()
	ce.lastProgressMsg = ""
	return ce.operationStartTime, "", true
}

func (ce *CommandExecutor) finish() {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.isRunning = false
}

func (ce *CommandExecutor) setProgress(msg string) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.lastProgressMsg = msg
}

// ExecuteCommand runs a command and returns a tea.Cmd that sends progress messages
func (ce *CommandExecutor) ExecuteCommand(cmd CommandType, files []string) tea.Cmd {
	switch cmd {
	case CommandSync:
		return ce.startOperation(cmd, files, ce.runSync)
	case CommandPull:
		return ce.startOperation(cmd, files, ce.runPull)
	case CommandPush:
		return ce.startOperation(cmd, files, ce.runPush)
	case CommandStatus:
		return ce.executeStatus(files)
	case CommandInit:
		return ce.executeInit(files)
	default:
		return func() tea.Msg {
			return CommandErrorMsg{
				Command: string(cmd),
				Error:   fmt.Errorf("unknown command: %s", cmd),
			}
		}
	}
}

// opResult is the outcome of a successful (or partially successful)
// operation.
type opResult struct {
	message string
	// count is the number of files actually synced.
	count int
}

type operationFunc func(ctx context.Context, files []string) (opResult, error)

// startOperation returns a command that announces the operation, runs it in
// the background with output captured, and reports completion or failure.
// A progress ticker runs alongside it.
func (ce *CommandExecutor) startOperation(cmd CommandType, files []string, run operationFunc) tea.Cmd {
	start, work, err := ce.prepareOperation(cmd, files, run)
	if err != nil {
		return func() tea.Msg {
			return CommandErrorMsg{Command: string(cmd), Error: err}
		}
	}
	return tea.Batch(
		tea.Sequence(start, work),
		progressTick(),
	)
}

// prepareOperation marks the executor busy and returns the start and work
// commands. work reports CommandCompleteMsg only when the operation fully
// succeeded, and CommandErrorMsg otherwise.
func (ce *CommandExecutor) prepareOperation(cmd CommandType, files []string, run operationFunc) (tea.Cmd, tea.Cmd, error) {
	startTime, running, ok := ce.begin(string(cmd))
	if !ok {
		return nil, nil, fmt.Errorf("cannot start %s: %s is still running", cmd, running)
	}

	files = append([]string(nil), files...)

	start := func() tea.Msg {
		return CommandStartMsg{Command: string(cmd), StartTime: startTime}
	}

	work := func() (msg tea.Msg) {
		defer ce.finish()
		defer func() {
			if r := recover(); r != nil {
				msg = CommandErrorMsg{
					Command:  string(cmd),
					Error:    fmt.Errorf("%s failed unexpectedly: %v", cmd, r),
					Duration: time.Since(startTime),
				}
			}
		}()

		var (
			res    opResult
			runErr error
		)
		// A capture failure only means output was not redirected; the
		// operation itself still ran.
		_ = captureOutput(ce.setProgress, func() {
			res, runErr = run(ce.ctx, files)
		})

		duration := time.Since(startTime)
		if runErr != nil {
			return CommandErrorMsg{
				Command:  string(cmd),
				Error:    runErr,
				Duration: duration,
				Count:    res.count,
			}
		}
		return CommandCompleteMsg{
			Command:  string(cmd),
			Duration: duration,
			Message:  res.message,
			Count:    res.count,
		}
	}

	return start, work, nil
}

func progressTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return ProgressTickMsg{}
	})
}

// runSync syncs the selected files. A full bidirectional sync prompts on
// stdin to resolve conflicts, which cannot work inside the TUI, so sync
// requires a selection and pushes the selected files.
func (ce *CommandExecutor) runSync(ctx context.Context, files []string) (opResult, error) {
	if len(files) == 0 {
		return opResult{}, fmt.Errorf("select files to sync (space to select), or use [p]ull / [P]ush for everything")
	}
	res, err := ce.pushFiles(ctx, files)
	if err != nil {
		return res, err
	}
	res.message = fmt.Sprintf("Synced %d file(s) to Notion", res.count)
	return res, nil
}

// runPush pushes the selected files, or every markdown file if nothing is
// selected.
func (ce *CommandExecutor) runPush(ctx context.Context, files []string) (opResult, error) {
	if len(files) == 0 {
		ce.setProgress("Pushing all files to Notion...")
		if err := ce.syncEngine.SyncAll(ctx, "push"); err != nil {
			return opResult{}, fmt.Errorf("push failed: %w", err)
		}
		all, err := ce.listMarkdownFiles(ce.markdownRoot())
		if err == nil {
			ce.markSynced(all)
		}
		return opResult{message: "Pushed all files to Notion", count: len(all)}, nil
	}

	res, err := ce.pushFiles(ctx, files)
	if err != nil {
		return res, err
	}
	res.message = fmt.Sprintf("Pushed %d file(s) to Notion", res.count)
	return res, nil
}

// pushFiles pushes each selected file and returns an error describing every
// failure if any file could not be pushed.
func (ce *CommandExecutor) pushFiles(ctx context.Context, files []string) (opResult, error) {
	paths, err := ce.resolveFiles(files)
	if err != nil {
		return opResult{}, err
	}
	if len(paths) == 0 {
		return opResult{}, fmt.Errorf("no markdown files in selection")
	}

	var pushed []string
	var failures []string
	for i, path := range paths {
		if ctx.Err() != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", ce.displayName(path), ctx.Err()))
			continue
		}
		ce.setProgress(fmt.Sprintf("Pushing %s (%d/%d)", ce.displayName(path), i+1, len(paths)))
		if err := ce.syncEngine.SyncFileToNotion(ctx, path); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", ce.displayName(path), err))
			continue
		}
		pushed = append(pushed, path)
	}

	ce.markSynced(pushed)

	res := opResult{count: len(pushed)}
	if len(failures) > 0 {
		return res, fmt.Errorf("pushed %d of %d file(s); failed: %s", len(pushed), len(paths), strings.Join(failures, "; "))
	}
	return res, nil
}

// runPull pulls the selected files from Notion using the notion_id in each
// file's frontmatter. With nothing selected it pulls the whole workspace.
func (ce *CommandExecutor) runPull(ctx context.Context, files []string) (opResult, error) {
	if len(files) == 0 {
		ce.setProgress("Pulling all pages from Notion...")
		if err := ce.syncEngine.SyncAll(ctx, "pull"); err != nil {
			return opResult{}, fmt.Errorf("pull failed: %w", err)
		}
		return opResult{message: "Pulled all pages from Notion"}, nil
	}

	paths, err := ce.resolveFiles(files)
	if err != nil {
		return opResult{}, err
	}
	if len(paths) == 0 {
		return opResult{}, fmt.Errorf("no markdown files in selection")
	}

	parser := markdown.NewParser()
	var pulled, skipped, failures []string
	for i, path := range paths {
		name := ce.displayName(path)
		if ctx.Err() != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", name, ctx.Err()))
			continue
		}

		notionID, err := notionIDForFile(parser, path)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if notionID == "" {
			skipped = append(skipped, name)
			continue
		}

		ce.setProgress(fmt.Sprintf("Pulling %s (%d/%d)", name, i+1, len(paths)))
		if err := ce.syncEngine.SyncNotionToFile(ctx, notionID, path); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		pulled = append(pulled, path)
	}

	ce.markSynced(pulled)

	res := opResult{count: len(pulled)}
	skippedNote := ""
	if len(skipped) > 0 {
		skippedNote = fmt.Sprintf("; skipped %d without notion_id (%s)", len(skipped), strings.Join(skipped, ", "))
	}
	if len(failures) > 0 {
		return res, fmt.Errorf("pulled %d of %d file(s)%s; failed: %s", len(pulled), len(paths), skippedNote, strings.Join(failures, "; "))
	}
	if len(pulled) == 0 {
		return res, fmt.Errorf("nothing pulled%s", skippedNote)
	}
	res.message = fmt.Sprintf("Pulled %d file(s) from Notion%s", len(pulled), skippedNote)
	return res, nil
}

// notionIDForFile returns the notion_id from a file's frontmatter, or "".
func notionIDForFile(parser markdown.Parser, path string) (string, error) {
	doc, err := parser.ParseFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to parse: %w", err)
	}
	fm, err := markdown.ExtractFrontmatter(doc.Metadata)
	if err != nil {
		return "", fmt.Errorf("invalid frontmatter: %w", err)
	}
	return fm.NotionID, nil
}

func (ce *CommandExecutor) markdownRoot() string {
	if ce.config != nil && ce.config.Directories.MarkdownRoot != "" {
		return ce.config.Directories.MarkdownRoot
	}
	return "."
}

// resolveFiles turns selected file list entries (paths relative to the
// markdown root, or absolute) into a sorted, de-duplicated list of markdown
// file paths. Selected directories expand to the markdown files inside.
func (ce *CommandExecutor) resolveFiles(files []string) ([]string, error) {
	root := ce.markdownRoot()
	seen := make(map[string]bool)
	var result []string

	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}

	for _, f := range files {
		p := f
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("cannot access %s: %w", f, err)
		}
		if info.IsDir() {
			inner, err := ce.listMarkdownFiles(p)
			if err != nil {
				return nil, err
			}
			for _, ip := range inner {
				add(ip)
			}
			continue
		}
		if strings.HasSuffix(p, ".md") {
			add(p)
		}
	}

	sort.Strings(result)
	return result, nil
}

// listMarkdownFiles returns the non-excluded markdown files under dir,
// skipping hidden directories.
func (ce *CommandExecutor) listMarkdownFiles(dir string) ([]string, error) {
	root := ce.markdownRoot()
	var patterns []string
	if ce.config != nil {
		patterns = ce.config.Directories.ExcludedPatterns
	}

	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != dir && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if util.IsExcluded(patterns, root, path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list markdown files in %s: %w", dir, err)
	}
	sort.Strings(files)
	return files, nil
}

// markSynced records successfully synced files in the staging area so the
// file list shows them as synced. It is best-effort: without a staging area
// (never initialised) nothing is recorded.
func (ce *CommandExecutor) markSynced(paths []string) {
	if ce.staging == nil || len(paths) == 0 {
		return
	}
	abs := make([]string, 0, len(paths))
	for _, p := range paths {
		if a, err := filepath.Abs(p); err == nil {
			abs = append(abs, a)
		}
	}
	_ = ce.staging.MarkSynced(abs)
}

// displayName returns path relative to the markdown root for messages.
func (ce *CommandExecutor) displayName(path string) string {
	root, err := filepath.Abs(ce.markdownRoot())
	if err != nil {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	if rel, err := filepath.Rel(root, abs); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return path
}

// executeStatus gets sync status
func (ce *CommandExecutor) executeStatus(_ []string) tea.Cmd {
	return func() tea.Msg {
		// Return status complete message
		return CommandCompleteMsg{
			Command: string(CommandStatus),
			Message: "Status check complete",
		}
	}
}

// executeInit initializes a new project
func (ce *CommandExecutor) executeInit(_ []string) tea.Cmd {
	return func() tea.Msg {
		startTime := time.Now()

		// Execute init synchronously and return the appropriate message

		// Check if already initialized
		if _, err := os.Stat("config.yaml"); err == nil {
			return CommandCompleteMsg{
				Command:  string(CommandInit),
				Duration: time.Since(startTime),
				Message:  "Project already initialized (config.yaml exists)",
			}
		}

		// Create config.yaml with defaults
		configContent := `# notion-md-sync configuration
notion:
  token: ""  # Set via NOTION_MD_SYNC_NOTION_TOKEN environment variable
  parent_page_id: ""  # Set via NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID environment variable

sync:
  direction: push
  conflict_resolution: newer

directories:
  markdown_root: ./docs
  excluded_patterns:
    - "*.tmp"
    - "node_modules/**"
    - ".git/**"

mapping:
  strategy: frontmatter
`

		if err := os.WriteFile("config.yaml", []byte(configContent), 0644); err != nil {
			return CommandErrorMsg{
				Command: string(CommandInit),
				Error:   fmt.Errorf("failed to create config.yaml: %w", err),
			}
		}

		// Create docs directory
		if err := os.MkdirAll("./docs", 0755); err != nil {
			return CommandErrorMsg{
				Command: string(CommandInit),
				Error:   fmt.Errorf("failed to create docs directory: %w", err),
			}
		}

		// Create .env.example
		envExampleContent := `# Copy this file to .env and fill in your actual values
NOTION_MD_SYNC_NOTION_TOKEN=your_integration_token_here
NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID=your_parent_page_id_here
`

		if err := os.WriteFile(".env.example", []byte(envExampleContent), 0644); err != nil {
			return CommandErrorMsg{
				Command: string(CommandInit),
				Error:   fmt.Errorf("failed to create .env.example: %w", err),
			}
		}

		// Create sample markdown file
		sampleContent := `---
title: "Welcome to notion-md-sync"
sync_enabled: true
---

# Welcome to notion-md-sync

This is a sample markdown file that demonstrates how notion-md-sync works.

## Getting Started

1. Edit this file
2. Run sync from the TUI or: notion-md-sync push
3. Check your Notion page!

## Features

- **Bidirectional sync** between markdown and Notion
- **Frontmatter support** for metadata
- **File watching** for automatic sync
- **Flexible configuration**

Happy syncing! 🚀
`

		if err := os.WriteFile("./docs/welcome.md", []byte(sampleContent), 0644); err != nil {
			return CommandErrorMsg{
				Command: string(CommandInit),
				Error:   fmt.Errorf("failed to create sample file: %w", err),
			}
		}

		return CommandCompleteMsg{
			Command:  string(CommandInit),
			Duration: time.Since(startTime),
			Message:  "Project initialized! Created config.yaml, .env.example, docs/ and sample file",
		}
	}
}

// Command Messages

// CommandStartMsg indicates a command has started
type CommandStartMsg struct {
	Command   string
	StartTime time.Time
}

// CommandProgressMsg provides progress updates
type CommandProgressMsg struct {
	Command     string
	CurrentFile string
	Message     string
	Progress    float64 // 0.0 to 1.0
}

// CommandCompleteMsg indicates a command has completed
type CommandCompleteMsg struct {
	Command  string
	Duration time.Duration
	Message  string
	// Count is the number of files synced by the command.
	Count int
}

// CommandErrorMsg indicates a command error
type CommandErrorMsg struct {
	Command  string
	Error    error
	Duration time.Duration
	// Count is the number of files that were synced before or despite the
	// failure (partial success).
	Count int
}

// CommandBatchMsg contains multiple command messages
type CommandBatchMsg struct {
	Messages []tea.Msg
}

// ProgressTickMsg is sent periodically to check for progress updates
type ProgressTickMsg struct{}
