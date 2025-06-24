package sync

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// ConflictResolver handles conflict resolution between local and remote content
type ConflictResolver struct {
	strategy string
}

// NewConflictResolver creates a new conflict resolver with the given strategy
func NewConflictResolver(strategy string) *ConflictResolver {
	return &ConflictResolver{
		strategy: strategy,
	}
}

// ResolveConflict resolves a conflict between local and remote content
func (cr *ConflictResolver) ResolveConflict(localContent, remoteContent, filePath string) (string, error) {
	switch cr.strategy {
	case "newer":
		return cr.resolveByNewer(localContent, remoteContent)
	case "notion_wins":
		return remoteContent, nil
	case "markdown_wins":
		return localContent, nil
	case "diff":
		return cr.resolveByDiff(localContent, remoteContent, filePath)
	default:
		return cr.resolveByDiff(localContent, remoteContent, filePath)
	}
}

// resolveByNewer resolves conflict by choosing the newer version
// For now, we'll default to showing diff since we need timestamp comparison
func (cr *ConflictResolver) resolveByNewer(localContent, remoteContent string) (string, error) {
	// TODO: Implement timestamp comparison when available
	// For now, fallback to diff resolution
	return cr.resolveByDiff(localContent, remoteContent, "")
}

// resolveByDiff shows a diff and lets the user choose
func (cr *ConflictResolver) resolveByDiff(localContent, remoteContent, filePath string) (string, error) {
	// Check if content is actually different
	if localContent == remoteContent {
		return localContent, nil
	}

	fmt.Printf("\n🔄 Conflict detected for: %s\n", filePath)
	fmt.Println("=" + strings.Repeat("=", 60) + "=")

	// Show diff
	if err := cr.showDiff(localContent, remoteContent); err != nil {
		return "", fmt.Errorf("failed to show diff: %w", err)
	}

	// Prompt user for choice
	fmt.Println("\nChoose resolution:")
	fmt.Println("  [l] Keep local (markdown) version")
	fmt.Println("  [r] Keep remote (Notion) version")
	fmt.Println("  [s] Skip this file")
	fmt.Print("\nYour choice [l/r/s]: ")

	reader := bufio.NewReader(os.Stdin)
	choice, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read user input: %w", err)
	}

	choice = strings.TrimSpace(strings.ToLower(choice))
	switch choice {
	case "l", "local":
		fmt.Println("✅ Using local version")
		return localContent, nil
	case "r", "remote":
		fmt.Println("✅ Using remote version")
		return remoteContent, nil
	case "s", "skip":
		fmt.Println("⏭️  Skipping file")
		return "", fmt.Errorf("user chose to skip file")
	default:
		fmt.Println("❌ Invalid choice, skipping file")
		return "", fmt.Errorf("invalid user choice")
	}
}

// showDiff displays a unified diff between local and remote content
func (cr *ConflictResolver) showDiff(localContent, remoteContent string) error {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(localContent, remoteContent, false)

	// Clean up for better readability
	diffs = dmp.DiffCleanupSemantic(diffs)

	printDiffHeader()

	for _, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffEqual:
			printContextLines(diff.Text)
		case diffmatchpatch.DiffDelete:
			printDeletedLines(diff.Text)
		case diffmatchpatch.DiffInsert:
			printInsertedLines(diff.Text)
		}
	}

	return nil
}

func printDiffHeader() {
	fmt.Println("\nDifferences:")
	fmt.Println("  + Added in remote (Notion)")
	fmt.Println("  - Removed in remote (Notion)")
	fmt.Println()
}

func printContextLines(text string) {
	lines := strings.Split(text, "\n")

	if len(lines) > 6 {
		printTruncatedContext(lines)
	} else {
		printFullContext(lines)
	}
}

func printTruncatedContext(lines []string) {
	// Show first 2 lines
	for i := 0; i < 2 && i < len(lines); i++ {
		if lines[i] != "" {
			fmt.Printf("   %s\n", lines[i])
		}
	}

	// Show ellipsis if needed
	if len(lines) > 4 {
		fmt.Println("   ...")
	}

	// Show last 2 lines
	for i := len(lines) - 2; i < len(lines); i++ {
		if i >= 0 && lines[i] != "" {
			fmt.Printf("   %s\n", lines[i])
		}
	}
}

func printFullContext(lines []string) {
	for _, line := range lines {
		if line != "" {
			fmt.Printf("   %s\n", line)
		}
	}
}

func printDeletedLines(text string) {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if line != "" {
			fmt.Printf(" - %s\n", line)
		}
	}
}

func printInsertedLines(text string) {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if line != "" {
			fmt.Printf(" + %s\n", line)
		}
	}
}

// HasConflict checks if there's a conflict between local and remote content
func HasConflict(localContent, remoteContent string) bool {
	return localContent != remoteContent
}
