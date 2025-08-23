package scanner

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rahulvramesh/cleanWithCli/internal/types"
	"github.com/rahulvramesh/cleanWithCli/internal/utils"
)

// Scanner performs the file system scanning
type Scanner struct {
	HomeDir string
	Results map[string]*types.ScanResult
	mu      sync.Mutex
}

// NewScanner creates a new scanner instance
func NewScanner() *Scanner {
	homeDir, _ := os.UserHomeDir()
	return &Scanner{
		HomeDir: homeDir,
		Results: make(map[string]*types.ScanResult),
	}
}

// ScanCacheFiles scans cache files
func (s *Scanner) ScanCacheFiles() *types.ScanResult {
	result := &types.ScanResult{
		Category: "Cache Files",
		Items:    []types.FileItem{},
	}

	cacheDirs := []string{
		filepath.Join(s.HomeDir, "Library", "Caches"),
		"/Library/Caches",
		filepath.Join(s.HomeDir, ".cache"),
	}

	for _, dir := range cacheDirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			size, _ := utils.GetDirSize(path)
			if size > 0 {
				result.Items = append(result.Items, types.FileItem{
					Path:  path,
					Size:  size,
					Name:  entry.Name(),
					IsDir: true,
				})
				result.Total += size
			}
		}
	}

	return result
}

// ScanLogFiles scans log files
func (s *Scanner) ScanLogFiles() *types.ScanResult {
	result := &types.ScanResult{
		Category: "Log Files",
		Items:    []types.FileItem{},
	}

	logDirs := []string{
		filepath.Join(s.HomeDir, "Library", "Logs"),
		"/Library/Logs",
		"/var/log",
	}

	for _, dir := range logDirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() && strings.Contains(d.Name(), ".log") {
				info, err := d.Info()
				if err == nil {
					result.Items = append(result.Items, types.FileItem{
						Path: path,
						Size: info.Size(),
						Name: d.Name(),
					})
					result.Total += info.Size()
				}
			}
			return nil
		})
	}

	return result
}

// ScanTrash scans trash directory
func (s *Scanner) ScanTrash() *types.ScanResult {
	result := &types.ScanResult{
		Category: "Trash",
		Items:    []types.FileItem{},
	}

	trashDir := filepath.Join(s.HomeDir, ".Trash")
	if _, err := os.Stat(trashDir); err != nil {
		return result
	}

	entries, err := os.ReadDir(trashDir)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		path := filepath.Join(trashDir, entry.Name())
		size, _ := utils.GetDirSize(path)
		result.Items = append(result.Items, types.FileItem{
			Path: path,
			Size: size,
			Name: entry.Name(),
		})
		result.Total += size
	}

	return result
}

// ScanDownloads scans old downloads
func (s *Scanner) ScanDownloads() *types.ScanResult {
	result := &types.ScanResult{
		Category: "Old Downloads",
		Items:    []types.FileItem{},
	}

	downloadsDir := filepath.Join(s.HomeDir, "Downloads")
	if _, err := os.Stat(downloadsDir); err != nil {
		return result
	}

	entries, err := os.ReadDir(downloadsDir)
	if err != nil {
		return result
	}

	cutoff := time.Now().AddDate(0, 0, -30) // 30 days ago

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(downloadsDir, entry.Name())
			size, _ := utils.GetDirSize(path)
			age := int(time.Since(info.ModTime()).Hours() / 24)

			result.Items = append(result.Items, types.FileItem{
				Path: path,
				Size: size,
				Name: entry.Name(),
				Age:  age,
			})
			result.Total += size
		}
	}

	return result
}
