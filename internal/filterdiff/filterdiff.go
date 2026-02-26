// Package filterdiff provides functionality for filtering unified diffs by
// filename patterns, similar to the filterdiff tool from patchutils.
package filterdiff

import (
	"path"
	"strings"

	"github.com/sourcegraph/go-diff/diff"
)

// Filter parses a unified diff and returns a filtered version containing only
// the file diffs whose filenames (after stripping stripComponents leading path
// components) match at least one include pattern and no exclude pattern.
//
// Patterns use path.Match syntax where '*' matches any non-separator character
// and '**' is not supported — however, for compatibility with filterdiff
// behavior, the matching is applied per-component so "chart/*" will match
// "chart/templates/deployment.yaml" by checking if the first component matches.
//
// The function uses sourcegraph/go-diff for robust diff parsing and printing.
func Filter(input string, stripComponents int, includes, excludes []string) (string, error) {
	fileDiffs, err := diff.ParseMultiFileDiff([]byte(input))
	if err != nil {
		return "", err
	}

	var filtered []*diff.FileDiff
	for _, fd := range fileDiffs {
		filename := bestFilename(fd)
		stripped := StripPathComponents(filename, stripComponents)

		if matchesAny(stripped, excludes) {
			continue
		}

		if matchesAny(stripped, includes) {
			filtered = append(filtered, fd)
		}
	}

	out, err := diff.PrintMultiFileDiff(filtered)
	if err != nil {
		return "", err
	}

	return string(out), nil
}

// bestFilename returns the most relevant filename for a FileDiff.
// For new files (OrigName is /dev/null), it returns NewName.
// Otherwise it returns OrigName.
func bestFilename(fd *diff.FileDiff) string {
	orig := fd.OrigName
	if orig == "/dev/null" || orig == "" {
		return fd.NewName
	}
	return orig
}

// StripPathComponents removes n leading path components from p.
// For example, StripPathComponents("a/chart/file.yaml", 1) returns "chart/file.yaml".
func StripPathComponents(p string, n int) string {
	for i := 0; i < n; i++ {
		idx := strings.Index(p, "/")
		if idx < 0 {
			return p
		}
		p = p[idx+1:]
	}
	return p
}

// matchesAny reports whether name matches any of the given patterns.
// Matching uses MatchWildcard which supports '*' matching across path separators,
// consistent with filterdiff's behavior.
func matchesAny(name string, patterns []string) bool {
	for _, pat := range patterns {
		if MatchWildcard(pat, name) {
			return true
		}
	}
	return false
}

// MatchWildcard reports whether name matches pattern. The '*' wildcard matches
// any sequence of characters including path separators, and '?' matches any
// single character. This matches filterdiff's fnmatch behavior without
// FNM_PATHNAME.
//
// For patterns like "chart/*", this will match "chart/sub/file.yaml" because
// '*' is greedy across '/'.
func MatchWildcard(pattern, name string) bool {
	// Try path.Match first for simple patterns without cross-separator wildcards.
	if matched, err := path.Match(pattern, name); err == nil && matched {
		return true
	}

	// Fall back to iterative wildcard matching that allows '*' to span separators.
	pi, ni := 0, 0
	starIdx := -1
	starMatch := 0
	for ni < len(name) {
		if pi < len(pattern) && (pattern[pi] == '?' || pattern[pi] == name[ni]) {
			pi++
			ni++
		} else if pi < len(pattern) && pattern[pi] == '*' {
			starIdx = pi
			starMatch = ni
			pi++
		} else if starIdx >= 0 {
			pi = starIdx + 1
			starMatch++
			ni = starMatch
		} else {
			return false
		}
	}
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}
	return pi == len(pattern)
}
