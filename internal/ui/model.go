package ui

import (
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rahulvramesh/cleanWithCli/internal/scanner"
	"github.com/rahulvramesh/cleanWithCli/internal/types"
)

// Model represents the application state
type Model struct {
	scanner        *scanner.Scanner
	state          string // "menu", "scanning", "results", "cleaning", "diskusage", "detail"
	menuChoice     int
	scanProgress   float64
	scanMessage    string
	spinner        spinner.Model
	progress       progress.Model
	table          table.Model
	results        map[string]*types.ScanResult
	totalSize      int64
	selectedItems  []string
	cleanProgress  float64
	width          int
	height         int
	err            error
	diskUsageTable table.Model
	// Detail view fields
	currentCategory string
	currentPath     []string // breadcrumb path
	detailItems     []types.FileItem
	detailChoice    int
	detailOffset    int // Scroll offset for detail view
	// Scanning view fields
	scanningPaths  []string // Recently scanned paths
	scanFoundItems int      // Number of items found
	scanTotalSize  int64    // Total size found so far
	// Multi-selection fields
	markedItems map[string]bool // Track marked items by path
}

// Initialize the model
func InitialModel() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		scanner:     scanner.NewScanner(),
		state:       "menu",
		spinner:     s,
		progress:    progress.New(progress.WithDefaultGradient()),
		markedItems: make(map[string]bool),
	}
}

// Init starts the spinner
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}
