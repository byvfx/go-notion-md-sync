package util

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrPathTraversal is returned when a path traversal attempt is detected
var ErrPathTraversal = errors.New("path traversal attempt detected")

// SecureJoin safely joins path elements and ensures the result is within the root directory.
// It prevents directory traversal attacks by validating that the final path
// remains within the specified root directory.
func SecureJoin(root string, elems ...string) (string, error) {
	// Clean the root path to ensure it's absolute and normalized
	root = filepath.Clean(root)

	// Join all elements
	joined := filepath.Join(elems...)

	// Clean the joined path to resolve . and .. elements
	cleaned := filepath.Clean(joined)

	// If the cleaned path is absolute, it's trying to escape
	if filepath.IsAbs(cleaned) {
		return "", ErrPathTraversal
	}

	// Create the full path
	fullPath := filepath.Join(root, cleaned)
	fullPath = filepath.Clean(fullPath)

	// Ensure the full path is still within the root directory
	// This check prevents directory traversal attacks
	if !strings.HasPrefix(fullPath, root+string(filepath.Separator)) && fullPath != root {
		return "", ErrPathTraversal
	}

	return fullPath, nil
}

// SanitizeFileName removes or replaces characters that could be problematic in file paths
func SanitizeFileName(name string) string {
	// First trim spaces
	name = strings.TrimSpace(name)

	// Replace path separators with underscores BEFORE handling dots
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")

	// Now remove leading dots and underscores to clean up the result
	name = strings.TrimLeft(name, "._")

	// Replace other potentially problematic characters
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "*", "_")
	name = strings.ReplaceAll(name, "?", "_")
	name = strings.ReplaceAll(name, "\"", "_")
	name = strings.ReplaceAll(name, "<", "_")
	name = strings.ReplaceAll(name, ">", "_")
	name = strings.ReplaceAll(name, "|", "_")

	// Trim any remaining spaces or underscores from ends
	name = strings.Trim(name, " _")

	// If the name is empty after sanitization, provide a default
	if name == "" {
		name = "untitled"
	}

	return name
}
