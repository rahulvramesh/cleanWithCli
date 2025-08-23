package types

import "github.com/charmbracelet/bubbles/table"

// ScanResult represents files found in a category
type ScanResult struct {
	Category string
	Items    []FileItem
	Total    int64
}

// FileItem represents a single file or directory
type FileItem struct {
	Path     string
	Size     int64
	Name     string
	Age      int // days old
	IsDir    bool
	Children []FileItem
}

// Messages
type ScanCompleteMsg struct {
	Results   map[string]*ScanResult
	TotalSize int64
}

type ScanProgressMsg struct {
	Percent float64
	Message string
	Path    string
	Size    int64
	Found   int
}

type CleanProgressMsg struct {
	Percent     float64
	Message     string
	Completed   int
	Total       int
	CurrentItem string
}

type CleanCompleteMsg struct {
	Freed int64
	Path  string // Path of the cleaned item
}

type BatchCleanCompleteMsg struct {
	Freed int64
	Paths []string // Paths of the cleaned items
}

type DiskUsageMsg struct {
	Table table.Model
}

type ErrMsg struct{ Err error }

func (e ErrMsg) Error() string { return e.Err.Error() }
