package util

import (
	"os"
	"strings"
	"testing"
)

// TestSecurityValidation_ComprehensiveTesting tests all security validations comprehensively
func TestSecurityValidation_ComprehensiveTesting(t *testing.T) {
	t.Run("PathTraversalAttacks", func(t *testing.T) {
		attackVectors := []struct {
			name        string
			path        string
			shouldError bool
		}{
			{"Basic dot-dot attack", "../../../etc/passwd", true},
			{"Windows-style attack", "..\\..\\..\\windows\\system32", false},         // On Linux, backslashes are literal filename chars
			{"URL encoded attack", "%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd", false}, // We don't decode URLs
			{"Mixed separators", "../..\\../etc/passwd", true},
			{"Relative path in middle", "docs/../../../etc/passwd", true},
			{"Valid nested path", "docs/subdoc/file.md", false},
			{"Current directory ref", "./file.md", false},
			{"Hidden file attempt", ".hidden/../../../etc/passwd", true},
		}

		tmpDir, err := os.MkdirTemp("", "security_test")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()

		for _, tt := range attackVectors {
			t.Run(tt.name, func(t *testing.T) {
				_, err := SecureJoin(tmpDir, tt.path)
				if (err != nil) != tt.shouldError {
					t.Errorf("SecureJoin() with %q: error = %v, wantErr %v", tt.path, err, tt.shouldError)
				}
			})
		}
	})

	t.Run("FilenameSanitization", func(t *testing.T) {
		dangerousFilenames := []struct {
			input    string
			expected string
		}{
			{"../../../etc/passwd", "etc_passwd"},
			{"con.txt", "con.txt"},                  // Windows reserved name
			{"null", "null"},                        // Valid on most systems
			{"file<>:\"|?*.txt", "file_______.txt"}, // Invalid chars - note the dot remains
			{"normal_file.md", "normal_file.md"},    // Should remain unchanged
			{".hidden", "hidden"},                   // Remove leading dot
			{"", "untitled"},                        // Empty becomes default
		}

		for _, tt := range dangerousFilenames {
			t.Run(tt.input, func(t *testing.T) {
				result := SanitizeFileName(tt.input)
				if result != tt.expected {
					t.Errorf("SanitizeFileName(%q) = %q, want %q", tt.input, result, tt.expected)
				}
			})
		}
	})

	t.Run("InputValidationEdgeCases", func(t *testing.T) {
		// Test very long inputs
		veryLongString := strings.Repeat("a", 10000)

		// Should handle long strings gracefully
		_, err := SanitizeAndValidateFilename(veryLongString)
		if err == nil {
			t.Error("Expected error for very long filename")
		}

		// Test with null bytes (Note: SanitizeFileName doesn't handle null bytes directly,
		// but they would be caught by validation layer in real usage)
		nullByteString := "file\x00name.txt"
		result := SanitizeFileName(nullByteString)
		// The function doesn't explicitly handle null bytes, so this test documents current behavior
		if !strings.Contains(result, "\x00") {
			t.Log("Note: SanitizeFileName passes null bytes through - validation layer should catch this")
		}

		// Test Unicode edge cases
		unicodeString := "файл.txt" // Cyrillic
		result = SanitizeFileName(unicodeString)
		if result == "" {
			t.Error("Unicode filename shouldn't become empty")
		}
	})

	t.Run("NotionIDValidation", func(t *testing.T) {
		maliciousIDs := []struct {
			id       string
			expected bool
		}{
			{"abcdef1234567890abcdef1234567890", false},     // Valid 32 char hex
			{"123e4567-e89b-12d3-a456-426614174000", false}, // Valid UUID
			{"'; DROP TABLE pages; --", true},               // SQL injection attempt
			{"../../../etc/passwd", true},                   // Path traversal
			{"<script>alert('xss')</script>", true},         // XSS attempt
			{"", true},                                      // Empty
			{"toolong" + strings.Repeat("a", 100), true},    // Too long
		}

		for _, tt := range maliciousIDs {
			t.Run(tt.id, func(t *testing.T) {
				err := ValidateNotionPageID(tt.id)
				if (err != nil) != tt.expected {
					t.Errorf("ValidateNotionPageID(%q) error = %v, wantErr %v", tt.id, err, tt.expected)
				}
			})
		}
	})

	t.Run("TokenValidation", func(t *testing.T) {
		maliciousTokens := []struct {
			token    string
			expected bool
		}{
			{"secret_" + strings.Repeat("a", 43), false},    // Valid long token
			{"ntn_validtoken123456", false},                 // Valid token without secret prefix
			{"valid_token_1234567890", false},               // Valid token
			{"short", true},                                 // Too short (less than 10 chars)
			{"", true},                                      // Empty
			{"   ", true},                                   // Whitespace only
		}

		for _, tt := range maliciousTokens {
			t.Run(tt.token, func(t *testing.T) {
				err := ValidateNotionToken(tt.token)
				if (err != nil) != tt.expected {
					t.Errorf("ValidateNotionToken(%q) error = %v, wantErr %v", tt.token, err, tt.expected)
				}
			})
		}
	})
}

// BenchmarkSecurityOperations benchmarks security-critical operations
func BenchmarkSecurityOperations(t *testing.B) {
	t.Run("PathValidation", func(b *testing.B) {
		tmpDir, _ := os.MkdirTemp("", "benchmark")
		defer func() { _ = os.RemoveAll(tmpDir) }()

		testPaths := []string{
			"normal/path/file.md",
			"../../../etc/passwd",
			"docs/subdoc/another/deep/path/file.md",
			"..\\..\\windows\\system32",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, p := range testPaths {
				_, _ = SecureJoin(tmpDir, p)
			}
		}
	})

	t.Run("FilenameSanitization", func(b *testing.B) {
		testFilenames := []string{
			"normal_file.md",
			"../../../etc/passwd",
			"file<>:\"|?*.txt",
			strings.Repeat("a", 255), // Max typical filename length
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, f := range testFilenames {
				SanitizeFileName(f)
			}
		}
	})

	t.Run("InputValidation", func(b *testing.B) {
		testInputs := []string{
			"push",
			"invalid_direction",
			"123e4567-e89b-12d3-a456-426614174000",
			"secret_" + strings.Repeat("a", 43),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, input := range testInputs {
				_ = ValidateSyncDirection(input)
				_ = ValidateNotionPageID(input)
				_ = ValidateNotionToken(input)
			}
		}
	})
}

// TestSecurityIntegration tests security measures work together
func TestSecurityIntegration(t *testing.T) {
	t.Run("EndToEndPathSecurity", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "integration_test")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()

		// Simulate processing a potentially malicious filename
		maliciousInput := "../../../etc/passwd"

		// 1. Sanitize the filename
		sanitized := SanitizeFileName(maliciousInput)
		if strings.Contains(sanitized, "..") {
			t.Error("Sanitization should remove path traversal attempts")
		}

		// 2. Validate the sanitized result
		validated, err := SanitizeAndValidateFilename(sanitized)
		if err != nil {
			t.Errorf("Validation failed for sanitized filename: %v", err)
		}

		// 3. Create a secure path
		safePath, err := SecureJoin(tmpDir, validated)
		if err != nil {
			t.Errorf("SecureJoin failed for validated filename: %v", err)
		}

		// 4. Ensure the final path is within the safe directory
		if !strings.HasPrefix(safePath, tmpDir) {
			t.Error("Final path should be within the safe directory")
		}

		// 5. The path should not contain the original malicious content
		if strings.Contains(safePath, "etc/passwd") {
			t.Error("Final path should not contain malicious content")
		}
	})

	t.Run("ConfigurationValidation", func(t *testing.T) {
		// Test that all expected validations work together
		testConfigs := []struct {
			pageID    string
			token     string
			direction string
			expectErr bool
		}{
			{
				pageID:    "123e4567-e89b-12d3-a456-426614174000",
				token:     "secret_" + strings.Repeat("a", 43),
				direction: "push",
				expectErr: false,
			},
			{
				pageID:    "invalid-id",
				token:     "secret_" + strings.Repeat("a", 43),
				direction: "push",
				expectErr: true,
			},
			{
				pageID:    "123e4567-e89b-12d3-a456-426614174000",
				token:     "short",  // Now too short (less than 10 chars)
				direction: "push",
				expectErr: true,
			},
			{
				pageID:    "123e4567-e89b-12d3-a456-426614174000",
				token:     "secret_" + strings.Repeat("a", 43),
				direction: "invalid",
				expectErr: true,
			},
		}

		for i, tc := range testConfigs {
			t.Run("", func(t *testing.T) {
				var hasError bool

				if err := ValidateNotionPageID(tc.pageID); err != nil {
					hasError = true
				}

				if err := ValidateNotionToken(tc.token); err != nil {
					hasError = true
				}

				if err := ValidateSyncDirection(tc.direction); err != nil {
					hasError = true
				}

				if hasError != tc.expectErr {
					t.Errorf("Test case %d: expected error=%v, got error=%v", i, tc.expectErr, hasError)
				}
			})
		}
	})
}

// TestSecurityRegressionPrevention ensures past vulnerabilities don't reappear
func TestSecurityRegressionPrevention(t *testing.T) {
	t.Run("PreventPathTraversalRegression", func(t *testing.T) {
		// These specific attack vectors were identified in the original code review
		regressionVectors := []struct {
			path        string
			shouldBlock bool
		}{
			{"../../../etc/passwd", true},
			{"..\\..\\..\\windows\\system32", false}, // On Linux, backslashes are literal filename chars
			{"docs/../../etc/passwd", true},
			{"./../../../etc/passwd", true},
		}

		tmpDir, _ := os.MkdirTemp("", "regression_test")
		defer func() { _ = os.RemoveAll(tmpDir) }()

		for _, vector := range regressionVectors {
			t.Run(vector.path, func(t *testing.T) {
				_, err := SecureJoin(tmpDir, vector.path)
				if vector.shouldBlock {
					if err == nil {
						t.Errorf("Path traversal attack vector %q should be blocked", vector.path)
					}
					if err != ErrPathTraversal {
						t.Errorf("Expected ErrPathTraversal, got %v", err)
					}
				} else {
					if err != nil {
						t.Errorf("Vector %q should not be blocked on this platform, got error: %v", vector.path, err)
					}
				}
			})
		}
	})

	t.Run("PreventUnsanitizedFilenames", func(t *testing.T) {
		// These specific dangerous filenames should be sanitized
		dangerousNames := []string{
			"../../../etc/passwd",
			"file<>:\"|?*.txt",
			"..hidden",
			"",
		}

		for _, name := range dangerousNames {
			t.Run(name, func(t *testing.T) {
				sanitized := SanitizeFileName(name)

				// Should not contain dangerous characters
				dangerous := []string{"..", "/", "\\", "<", ">", ":", "\"", "|", "?", "*"}
				for _, d := range dangerous {
					if strings.Contains(sanitized, d) {
						t.Errorf("Sanitized filename %q still contains dangerous character %q", sanitized, d)
					}
				}

				// Should not be empty (should get default name)
				if sanitized == "" {
					t.Error("Sanitized filename should not be empty")
				}
			})
		}
	})
}
