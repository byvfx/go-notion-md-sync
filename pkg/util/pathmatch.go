package util

import (
	"path"
	"path/filepath"
	"strings"
)

// MatchExcludePattern reports whether relPath (a path relative to the
// markdown root, using either slash or OS separators) matches the exclude
// pattern.
//
// Matching rules, loosely modelled on .gitignore:
//   - A pattern without a slash (e.g. "*.tmp", "node_modules") matches the
//     name of any component of relPath.
//   - A pattern of the form "dir/**" where dir has no slash (e.g.
//     ".git/**", "node_modules/**") matches dir and anything beneath it, at
//     any depth in the tree.
//   - "**" as a whole path segment matches zero or more segments, so
//     "a/**/b" matches "a/b", "a/x/b" and "a/x/y/b".
//   - Any other pattern is matched against the whole relative path, segment
//     by segment, using path.Match semantics.
//
// A leading "/" or "./" on the pattern is ignored, as is a trailing "/".
// Malformed patterns never match.
func MatchExcludePattern(pattern, relPath string) bool {
	pattern = normalizePattern(pattern)
	if pattern == "" {
		return false
	}

	rel := normalizeRelPath(relPath)
	if rel == "" {
		return false
	}
	segments := strings.Split(rel, "/")

	// No slash: match the base name of any path component.
	if !strings.Contains(pattern, "/") {
		for _, seg := range segments {
			if ok, err := path.Match(pattern, seg); err == nil && ok {
				return true
			}
		}
		return false
	}

	// "dir/**" with a single-segment dir matches that directory anywhere.
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		if prefix != "" && !strings.Contains(prefix, "/") && prefix != "**" {
			pattern = "**/" + pattern
		}
	}

	return matchSegments(strings.Split(pattern, "/"), segments)
}

// IsExcluded reports whether filePath matches any of the exclude patterns.
// The path is made relative to root first (root may be "" when filePath is
// already relative). A path is also excluded when any of its parent
// directories matches a pattern, so "drafts" excludes "drafts/a/b.md".
func IsExcluded(patterns []string, root, filePath string) bool {
	if len(patterns) == 0 {
		return false
	}

	rel := relativeTo(root, filePath)
	rel = normalizeRelPath(rel)
	if rel == "" {
		return false
	}

	segments := strings.Split(rel, "/")
	for _, pattern := range patterns {
		for i := len(segments); i > 0; i-- {
			if MatchExcludePattern(pattern, strings.Join(segments[:i], "/")) {
				return true
			}
		}
	}
	return false
}

// relativeTo returns filePath relative to root when possible, otherwise the
// cleaned filePath itself.
func relativeTo(root, filePath string) string {
	if root == "" {
		return filePath
	}

	base, target := root, filePath
	if filepath.IsAbs(base) != filepath.IsAbs(target) {
		if !filepath.IsAbs(target) {
			// A relative path paired with an absolute root is taken to be
			// relative to that root already.
			return filePath
		}
		absBase, err := filepath.Abs(base)
		if err != nil {
			return filePath
		}
		base = absBase
	}

	rel, err := filepath.Rel(base, target)
	if err != nil {
		return filePath
	}
	return rel
}

func normalizePattern(pattern string) string {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	for strings.HasPrefix(pattern, "./") {
		pattern = strings.TrimPrefix(pattern, "./")
	}
	pattern = strings.TrimPrefix(pattern, "/")
	pattern = strings.TrimSuffix(pattern, "/")
	return pattern
}

func normalizeRelPath(p string) string {
	p = path.Clean(filepath.ToSlash(p))
	if p == "." || p == "/" {
		return ""
	}
	p = strings.TrimPrefix(p, "/")

	// Drop leading ".." segments for paths outside the root; component
	// based patterns can still apply to the remainder.
	for p == ".." || strings.HasPrefix(p, "../") {
		p = strings.TrimPrefix(strings.TrimPrefix(p, ".."), "/")
	}
	return p
}

// matchSegments matches path segments against pattern segments where a
// "**" pattern segment matches zero or more path segments.
func matchSegments(pattern, segments []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			// Collapse consecutive globstars.
			for len(pattern) > 0 && pattern[0] == "**" {
				pattern = pattern[1:]
			}
			if len(pattern) == 0 {
				return true
			}
			for i := 0; i <= len(segments); i++ {
				if matchSegments(pattern, segments[i:]) {
					return true
				}
			}
			return false
		}

		if len(segments) == 0 {
			return false
		}
		ok, err := path.Match(pattern[0], segments[0])
		if err != nil || !ok {
			return false
		}
		pattern = pattern[1:]
		segments = segments[1:]
	}
	return len(segments) == 0
}
