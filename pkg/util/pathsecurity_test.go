package util

import (
	"path/filepath"
	"testing"
)

func TestSecureJoin(t *testing.T) {
	tests := []struct {
		name    string
		root    string
		elems   []string
		want    string
		wantErr bool
	}{
		{
			name:    "simple join",
			root:    "/home/user/docs",
			elems:   []string{"folder", "file.md"},
			want:    filepath.Join("/home/user/docs", "folder", "file.md"),
			wantErr: false,
		},
		{
			name:    "path traversal with ..",
			root:    "/home/user/docs",
			elems:   []string{"../../../etc", "passwd"},
			want:    "",
			wantErr: true,
		},
		{
			name:    "path traversal with absolute path",
			root:    "/home/user/docs",
			elems:   []string{"/etc/passwd"},
			want:    "",
			wantErr: true,
		},
		{
			name:    "valid nested path",
			root:    "/home/user/docs",
			elems:   []string{"level1", "level2", "file.md"},
			want:    filepath.Join("/home/user/docs", "level1", "level2", "file.md"),
			wantErr: false,
		},
		{
			name:    "path with dots that stay within root",
			root:    "/home/user/docs",
			elems:   []string{"folder", "..", "another", "file.md"},
			want:    filepath.Join("/home/user/docs", "another", "file.md"),
			wantErr: false,
		},
		{
			name:    "empty elements",
			root:    "/home/user/docs",
			elems:   []string{},
			want:    "/home/user/docs",
			wantErr: false,
		},
		{
			name:    "path traversal with backslash filename",
			root:    "/home/user/docs",
			elems:   []string{"..\\..\\etc", "passwd"},
			want:    filepath.Join("/home/user/docs", "..\\..\\etc", "passwd"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SecureJoin(tt.root, tt.elems...)
			if (err != nil) != tt.wantErr {
				t.Errorf("SecureJoin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SecureJoin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normal filename",
			input: "document.md",
			want:  "document.md",
		},
		{
			name:  "filename with slashes",
			input: "path/to/file.md",
			want:  "path_to_file.md",
		},
		{
			name:  "filename with backslashes",
			input: "path\\to\\file.md",
			want:  "path_to_file.md",
		},
		{
			name:  "filename with dots",
			input: "../../../etc/passwd",
			want:  "etc_passwd",
		},
		{
			name:  "filename with special characters",
			input: "file:*?\"<>|name.md",
			want:  "file_______name.md",
		},
		{
			name:  "filename with spaces",
			input: "  spaces around  ",
			want:  "spaces around",
		},
		{
			name:  "empty filename",
			input: "",
			want:  "untitled",
		},
		{
			name:  "only special characters",
			input: "...",
			want:  "untitled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeFileName(tt.input); got != tt.want {
				t.Errorf("SanitizeFileName() = %v, want %v", got, tt.want)
			}
		})
	}
}
