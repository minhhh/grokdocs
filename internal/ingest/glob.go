package ingest

import (
	"path/filepath"
	"strings"
)

// matchGlob reports whether path matches pattern, supporting ** for recursive
// directory matching (tsconfig-style). Without ** it falls back to basename
// matching for backward compatibility.
func matchGlob(path, pattern string) bool {
	if !strings.Contains(pattern, "**") {
		if strings.Contains(pattern, "/") {
			matched, err := filepath.Match(pattern, path)
			return err == nil && matched
		}
		base := filepath.Base(path)
		matched, err := filepath.Match(pattern, base)
		return err == nil && matched
	}

	path = filepath.ToSlash(path)
	pattern = filepath.ToSlash(pattern)

	patParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")
	return matchGlobParts(pathParts, patParts)
}

func matchGlobParts(pathParts, patParts []string) bool {
	if len(patParts) == 0 {
		return len(pathParts) == 0
	}

	if len(pathParts) == 0 {
		return allDoubleStar(patParts)
	}

	part := patParts[0]

	if part == "**" {
		for i := 0; i <= len(pathParts); i++ {
			if matchGlobParts(pathParts[i:], patParts[1:]) {
				return true
			}
		}
		return false
	}

	matched, err := filepath.Match(part, pathParts[0])
	if err != nil || !matched {
		return false
	}
	return matchGlobParts(pathParts[1:], patParts[1:])
}

func allDoubleStar(parts []string) bool {
	return len(parts) == 0 || (parts[0] == "**" && allDoubleStar(parts[1:]))
}
