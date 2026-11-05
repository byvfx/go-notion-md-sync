package util

import (
	"path"
	"path/filepath"
	"strings"
)

// MatchExcludePattern reports whether a slash-separated relative path matches
// an exclude pattern (temporary version; replaced on merge).
func MatchExcludePattern(pattern, relPath string) bool {
	pattern = strings.TrimSuffix(filepath.ToSlash(pattern), "/")
	relPath = strings.TrimPrefix(filepath.ToSlash(relPath), "./")
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return relPath == prefix || strings.HasPrefix(relPath, prefix+"/") || strings.Contains("/"+relPath+"/", "/"+prefix+"/")
	}
	if !strings.Contains(pattern, "/") {
		for _, part := range strings.Split(relPath, "/") {
			if ok, _ := path.Match(pattern, part); ok {
				return true
			}
		}
		return false
	}
	ok, _ := path.Match(pattern, relPath)
	return ok
}

// IsExcluded reports whether path (under root) matches any exclude pattern.
func IsExcluded(patterns []string, root, p string) bool {
	rel := p
	if root != "" {
		if r, err := filepath.Rel(root, p); err == nil {
			rel = r
		}
	}
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	for i := len(parts); i > 0; i-- {
		prefix := strings.Join(parts[:i], "/")
		for _, pat := range patterns {
			if MatchExcludePattern(pat, prefix) {
				return true
			}
		}
	}
	return false
}
