package util

import (
	"os"
	"testing"
)

func TestValidateRequired(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		fieldName string
		wantErr   bool
	}{
		{"valid input", "test", "field", false},
		{"empty string", "", "field", true},
		{"whitespace only", "   ", "field", true},
		{"valid with spaces", " test ", "field", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequired(tt.input, tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRequired() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSyncDirection(t *testing.T) {
	tests := []struct {
		name      string
		direction string
		wantErr   bool
	}{
		{"valid push", "push", false},
		{"valid pull", "pull", false},
		{"valid bidirectional", "bidirectional", false},
		{"valid push uppercase", "PUSH", false},
		{"valid pull with spaces", " pull ", false},
		{"invalid direction", "invalid", true},
		{"empty direction", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSyncDirection(tt.direction)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSyncDirection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFilePath(t *testing.T) {
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	_ = tmpFile.Close()

	tests := []struct {
		name      string
		path      string
		mustExist bool
		wantErr   bool
	}{
		{"valid existing file", tmpFile.Name(), true, false},
		{"valid non-existing file", "/tmp/nonexistent", false, false},
		{"non-existing file with mustExist", "/tmp/nonexistent", true, true},
		{"path with traversal", "../../../etc/passwd", false, true},
		{"empty path", "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilePath(tt.path, tt.mustExist)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDirectoryPath(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a temporary file (not directory) for testing
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	_ = tmpFile.Close()

	tests := []struct {
		name      string
		path      string
		mustExist bool
		wantErr   bool
	}{
		{"valid existing directory", tmpDir, true, false},
		{"valid non-existing directory", "/tmp/nonexistent", false, false},
		{"non-existing directory with mustExist", "/tmp/nonexistent", true, true},
		{"file instead of directory", tmpFile.Name(), true, true},
		{"empty path", "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDirectoryPath(tt.path, tt.mustExist)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDirectoryPath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateNotionPageID(t *testing.T) {
	tests := []struct {
		name    string
		pageID  string
		wantErr bool
	}{
		{"valid UUID format", "123e4567-e89b-12d3-a456-426614174000", false},
		{"valid 32 char hex", "abcdef1234567890abcdef1234567890", false},
		{"invalid short", "123", true},
		{"invalid characters", "invalid-page-id", true},
		{"empty page ID", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNotionPageID(tt.pageID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNotionPageID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateNotionToken(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{"valid token", "secret_abcdefghijklmnopqrstuvwxyz1234567890123", false},
		{"invalid prefix", "invalid_abcdefghijklmnopqrstuvwxyz1234567890123", true},
		{"too short", "secret_short", true},
		{"empty token", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNotionToken(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNotionToken() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid http", "http://example.com", false},
		{"valid https", "https://example.com", false},
		{"invalid no protocol", "example.com", true},
		{"invalid protocol", "ftp://example.com", true},
		{"empty URL", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateStringLength(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		fieldName string
		minLen    int
		maxLen    int
		wantErr   bool
	}{
		{"valid length", "test", "field", 1, 10, false},
		{"too short", "a", "field", 5, 10, true},
		{"too long", "verylongstring", "field", 1, 5, true},
		{"exact min", "test", "field", 4, 10, false},
		{"exact max", "test", "field", 1, 4, false},
		{"no max limit", "verylongstring", "field", 1, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStringLength(tt.input, tt.fieldName, tt.minLen, tt.maxLen)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStringLength() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePositiveInt(t *testing.T) {
	tests := []struct {
		name      string
		value     int
		fieldName string
		wantErr   bool
	}{
		{"positive value", 5, "field", false},
		{"zero value", 0, "field", true},
		{"negative value", -1, "field", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePositiveInt(tt.value, tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePositiveInt() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateIntRange(t *testing.T) {
	tests := []struct {
		name      string
		value     int
		fieldName string
		min       int
		max       int
		wantErr   bool
	}{
		{"value in range", 5, "field", 1, 10, false},
		{"value at min", 1, "field", 1, 10, false},
		{"value at max", 10, "field", 1, 10, false},
		{"value below min", 0, "field", 1, 10, true},
		{"value above max", 11, "field", 1, 10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIntRange(tt.value, tt.fieldName, tt.min, tt.max)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIntRange() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeAndValidateFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
		wantErr  bool
	}{
		{"normal filename", "document.md", "document.md", false},
		{"filename with slashes", "path/to/file.md", "path_to_file.md", false},
		{"filename with dots", "../../../etc/passwd", "etc_passwd", false},
		{"empty filename", "", "", true},
		{"very long filename", string(make([]byte, 300)), "", true}, // Too long after generation
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SanitizeAndValidateFilename(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizeAndValidateFilename() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("SanitizeAndValidateFilename() = %v, want %v", got, tt.want)
			}
		})
	}
}
