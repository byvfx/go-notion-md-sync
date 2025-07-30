package util

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Validation errors
var (
	ErrEmptyInput      = errors.New("input cannot be empty")
	ErrInvalidPath     = errors.New("invalid file path")
	ErrInvalidURL      = errors.New("invalid URL format")
	ErrInvalidPageID   = errors.New("invalid Notion page ID format")
	ErrInvalidToken    = errors.New("invalid token format")
	ErrInvalidDuration = errors.New("invalid duration format")
)

// ValidSyncDirections contains all valid sync directions
var ValidSyncDirections = []string{"push", "pull", "bidirectional"}

// NotionPageIDRegex matches valid Notion page IDs (32 hex chars or UUID format)
var NotionPageIDRegex = regexp.MustCompile(`^[a-fA-F0-9]{32}$|^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

// NotionTokenRegex matches valid Notion integration tokens (deprecated - not used anymore)
// We now accept any token format and let Notion validate it
var NotionTokenRegex = regexp.MustCompile(`^.{10,}$`)

// ValidateRequired validates that a string input is not empty
func ValidateRequired(input, fieldName string) error {
	if strings.TrimSpace(input) == "" {
		return fmt.Errorf("%s is required: %w", fieldName, ErrEmptyInput)
	}
	return nil
}

// ValidateSyncDirection validates sync direction parameter
func ValidateSyncDirection(direction string) error {
	if err := ValidateRequired(direction, "sync direction"); err != nil {
		return err
	}

	direction = strings.ToLower(strings.TrimSpace(direction))
	for _, valid := range ValidSyncDirections {
		if direction == valid {
			return nil
		}
	}

	return fmt.Errorf("invalid sync direction '%s', must be one of: %s",
		direction, strings.Join(ValidSyncDirections, ", "))
}

// ValidateFilePath validates that a file path is safe and exists
func ValidateFilePath(path string, mustExist bool) error {
	if err := ValidateRequired(path, "file path"); err != nil {
		return err
	}

	// Clean the path to resolve . and .. elements
	cleanPath := filepath.Clean(path)

	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path contains directory traversal: %w", ErrInvalidPath)
	}

	// Check if path exists when required
	if mustExist {
		if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", cleanPath)
		} else if err != nil {
			return fmt.Errorf("cannot access path: %w", err)
		}
	}

	return nil
}

// ValidateDirectoryPath validates that a directory path is safe and optionally exists
func ValidateDirectoryPath(path string, mustExist bool) error {
	if err := ValidateFilePath(path, mustExist); err != nil {
		return err
	}

	if mustExist {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("cannot access directory: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("path is not a directory: %s", path)
		}
	}

	return nil
}

// ValidateNotionPageID validates a Notion page ID format
func ValidateNotionPageID(pageID string) error {
	if err := ValidateRequired(pageID, "Notion page ID"); err != nil {
		return err
	}

	// Remove any hyphens for validation
	cleanID := strings.ReplaceAll(pageID, "-", "")

	if !NotionPageIDRegex.MatchString(pageID) && len(cleanID) != 32 {
		return fmt.Errorf("invalid Notion page ID format: %w", ErrInvalidPageID)
	}

	return nil
}

// ValidateNotionToken validates a Notion integration token format
func ValidateNotionToken(token string) error {
	if err := ValidateRequired(token, "Notion token"); err != nil {
		return err
	}

	// Accept any non-empty token - Notion will validate it when we try to use it
	// This is more user-friendly and accommodates different token formats
	if len(strings.TrimSpace(token)) < 10 {
		return fmt.Errorf("token seems too short: %w", ErrInvalidToken)
	}

	return nil
}

// ValidateURL validates a URL format (basic validation)
func ValidateURL(url string) error {
	if err := ValidateRequired(url, "URL"); err != nil {
		return err
	}

	url = strings.TrimSpace(url)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("URL must start with http:// or https://: %w", ErrInvalidURL)
	}

	return nil
}

// ValidateStringLength validates string length constraints
func ValidateStringLength(input, fieldName string, minLen, maxLen int) error {
	if err := ValidateRequired(input, fieldName); err != nil {
		return err
	}

	length := len(strings.TrimSpace(input))
	if length < minLen {
		return fmt.Errorf("%s must be at least %d characters long", fieldName, minLen)
	}
	if maxLen > 0 && length > maxLen {
		return fmt.Errorf("%s must be at most %d characters long", fieldName, maxLen)
	}

	return nil
}

// ValidatePositiveInt validates that an integer is positive
func ValidatePositiveInt(value int, fieldName string) error {
	if value <= 0 {
		return fmt.Errorf("%s must be positive (got %d)", fieldName, value)
	}
	return nil
}

// ValidateIntRange validates that an integer is within a specified range
func ValidateIntRange(value int, fieldName string, min, max int) error {
	if value < min || value > max {
		return fmt.Errorf("%s must be between %d and %d (got %d)", fieldName, min, max, value)
	}
	return nil
}

// SanitizeAndValidateFilename combines filename sanitization with validation
func SanitizeAndValidateFilename(filename string) (string, error) {
	if err := ValidateRequired(filename, "filename"); err != nil {
		return "", err
	}

	// Sanitize the filename
	sanitized := SanitizeFileName(filename)

	// Validate the result
	if err := ValidateStringLength(sanitized, "sanitized filename", 1, 255); err != nil {
		return "", err
	}

	return sanitized, nil
}
