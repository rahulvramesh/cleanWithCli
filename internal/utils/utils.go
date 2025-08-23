package utils

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/rahulvramesh/cleanWithCli/internal/types"
)

// GetDirSize calculates the total size of a directory
func GetDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// GetSortedCategories returns sorted category names from scan results
func GetSortedCategories(results map[string]*types.ScanResult) []string {
	categories := make([]string, 0, len(results))
	for category := range results {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	return categories
}

// TruncatePath truncates a path if it's too long
func TruncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return path[:maxLen-3] + "..."
}

// FormatFileSize formats file size using humanize
func FormatFileSize(size int64) string {
	return humanize.Bytes(uint64(size))
}

// IsProjectDir checks if a directory contains project files
func IsProjectDir(dirPath string) bool {
	projectFiles := []string{"package.json", "Cargo.toml", "pom.xml", "build.gradle", "Makefile", "CMakeLists.txt"}
	for _, pf := range projectFiles {
		if _, err := os.Stat(filepath.Join(dirPath, pf)); err == nil {
			return true
		}
	}
	return false
}

// ShouldSkipDir checks if a directory should be skipped during scanning
func ShouldSkipDir(path string) bool {
	skipPatterns := []string{
		"/Library/", "/System/", "/.Trash/", "/Applications/",
		"/System/Library/", "/usr/", "/bin/", "/sbin/",
	}

	for _, pattern := range skipPatterns {
		if strings.Contains(path, pattern) {
			// Allow some Library subdirectories that might contain user projects
			if strings.Contains(path, "/Library/") &&
				(strings.Contains(path, "/Documents/") || strings.Contains(path, "/Desktop/")) {
				continue
			}
			return true
		}
	}
	return false
}

// WalkDirWithProgress walks a directory and sends progress updates
func WalkDirWithProgress(root string, progressChan chan<- types.ScanProgressMsg, fn func(path string, d fs.DirEntry, err error) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err := fn(path, d, err); err != nil {
			return err
		}

		// Send progress update occasionally
		if progressChan != nil && strings.HasSuffix(path, "/") == false {
			select {
			case progressChan <- types.ScanProgressMsg{
				Path:    path,
				Message: fmt.Sprintf("Scanning: %s", filepath.Base(path)),
			}:
			default:
			}
		}

		return nil
	})
}
