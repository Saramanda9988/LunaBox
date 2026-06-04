package importpath

import (
	"path/filepath"
	"strings"
)

// Normalize returns a stable Windows import-path key for duplicate checks.
func Normalize(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}

	cleaned := filepath.Clean(trimmed)
	if abs, err := filepath.Abs(cleaned); err == nil {
		cleaned = abs
	}
	cleaned = strings.ReplaceAll(cleaned, "/", `\`)
	return strings.ToLower(cleaned)
}

func ContainsNormalized(parentPath string, childPath string) bool {
	parentPath = strings.TrimRight(parentPath, `\`)
	childPath = strings.TrimRight(childPath, `\`)
	if parentPath == "" || childPath == "" || parentPath == childPath {
		return false
	}
	return strings.HasPrefix(childPath, parentPath+`\`)
}

func Conflicts(pathA string, pathB string) bool {
	normalizedA := Normalize(pathA)
	normalizedB := Normalize(pathB)
	if normalizedA == "" || normalizedB == "" {
		return false
	}
	if normalizedA == normalizedB {
		return true
	}
	return ContainsNormalized(normalizedA, normalizedB) || ContainsNormalized(normalizedB, normalizedA)
}
