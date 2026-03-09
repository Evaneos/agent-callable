package spec

import (
	"path/filepath"
	"strings"
)

// IsUnderWritableDir checks if path is /dev/null or under one of the given directories.
func IsUnderWritableDir(path string, dirs []string) bool {
	if path == "/dev/null" {
		return true
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, dir := range dirs {
		dirAbs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if strings.HasPrefix(abs, dirAbs+"/") || abs == dirAbs {
			return true
		}
	}
	return false
}
