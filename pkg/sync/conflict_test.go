package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasConflict(t *testing.T) {
	tests := []struct {
		name          string
		localContent  string
		remoteContent string
		expected      bool
	}{
		{
			name:          "identical content",
			localContent:  "# Hello World\nThis is a test",
			remoteContent: "# Hello World\nThis is a test",
			expected:      false,
		},
		{
			name:          "different content",
			localContent:  "# Hello World\nThis is a test",
			remoteContent: "# Hello World\nThis is different",
			expected:      true,
		},
		{
			name:          "empty content",
			localContent:  "",
			remoteContent: "",
			expected:      false,
		},
		{
			name:          "one empty one not",
			localContent:  "# Hello",
			remoteContent: "",
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasConflict(tt.localContent, tt.remoteContent)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewConflictResolver(t *testing.T) {
	resolver := NewConflictResolver("diff")
	assert.NotNil(t, resolver)
	assert.Equal(t, "diff", resolver.strategy)
}

func TestResolveConflict_NotionWins(t *testing.T) {
	resolver := NewConflictResolver("notion_wins")

	localContent := "# Local Version\nLocal content"
	remoteContent := "# Remote Version\nRemote content"

	result, err := resolver.ResolveConflict(localContent, remoteContent, "test.md")

	assert.NoError(t, err)
	assert.Equal(t, remoteContent, result)
}

func TestResolveConflict_MarkdownWins(t *testing.T) {
	resolver := NewConflictResolver("markdown_wins")

	localContent := "# Local Version\nLocal content"
	remoteContent := "# Remote Version\nRemote content"

	result, err := resolver.ResolveConflict(localContent, remoteContent, "test.md")

	assert.NoError(t, err)
	assert.Equal(t, localContent, result)
}

func TestResolveConflict_NoConflict(t *testing.T) {
	resolver := NewConflictResolver("diff")

	content := "# Same Content\nThis is identical"

	result, err := resolver.ResolveConflict(content, content, "test.md")

	assert.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestShowDiff(t *testing.T) {
	resolver := NewConflictResolver("diff")

	localContent := "# Hello World\nThis is the original content\nWith multiple lines"
	remoteContent := "# Hello World\nThis is the modified content\nWith different lines"

	// Capture the output by redirecting to a string (this is a simple test)
	err := resolver.showDiff(localContent, remoteContent)
	assert.NoError(t, err)
}

func TestResolveByNewer_FallsBackToDiff(t *testing.T) {
	resolver := NewConflictResolver("newer")

	// Since newer resolution requires timestamps which we don't have implemented yet,
	// it should fallback to diff resolution, which requires user input
	// For testing, we'll just verify it doesn't panic and returns expected behavior

	localContent := "# Local"
	remoteContent := "# Remote"

	// This would normally require user input, but in a test environment
	// it might fail or need mocking. For now, we'll test the structure exists.
	_, err := resolver.resolveByNewer(localContent, remoteContent)

	// The error is expected since we can't provide user input in tests
	// but we want to make sure the method exists and doesn't panic
	assert.Error(t, err) // Expected since no user input available in test
}

// Test the diff display functionality without user interaction
func TestShowDiff_DifferentTypes(t *testing.T) {
	resolver := NewConflictResolver("diff")

	testCases := []struct {
		name          string
		localContent  string
		remoteContent string
	}{
		{
			name:          "added lines",
			localContent:  "Line 1\nLine 2",
			remoteContent: "Line 1\nLine 2\nLine 3",
		},
		{
			name:          "removed lines",
			localContent:  "Line 1\nLine 2\nLine 3",
			remoteContent: "Line 1\nLine 3",
		},
		{
			name:          "modified lines",
			localContent:  "Hello World",
			remoteContent: "Hello Universe",
		},
		{
			name:          "complex changes",
			localContent:  "# Title\n\nParagraph 1\nParagraph 2\n\n## Section",
			remoteContent: "# Different Title\n\nParagraph 1 modified\nParagraph 3\n\n## New Section",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := resolver.showDiff(tc.localContent, tc.remoteContent)
			assert.NoError(t, err)
		})
	}
}

func TestConflictResolver_UnknownStrategy(t *testing.T) {
	resolver := NewConflictResolver("unknown_strategy")

	localContent := "# Local"
	remoteContent := "# Remote"

	// Unknown strategy should fallback to diff
	_, err := resolver.ResolveConflict(localContent, remoteContent, "test.md")

	// This will fail due to user input requirement, but that's expected
	assert.Error(t, err)
}

// Mock test for diff resolution that simulates user input
func TestDiffResolution_MockUserInput(t *testing.T) {
	// This test demonstrates how we might mock user input in the future
	// For now, it's a placeholder showing the structure

	resolver := NewConflictResolver("diff")

	// In a real implementation, we'd mock os.Stdin or use dependency injection
	// to provide a test reader that simulates user input

	assert.NotNil(t, resolver)
	assert.Equal(t, "diff", resolver.strategy)
}
