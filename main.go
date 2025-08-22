package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			Background(lipgloss.Color("235")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("46"))

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226"))
)

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

// Scanner performs the file system scanning
type Scanner struct {
	HomeDir string
	Results map[string]*ScanResult
	mu      sync.Mutex
}

// Model represents the application state
type Model struct {
	scanner        *Scanner
	state          string // "menu", "scanning", "results", "cleaning", "diskusage", "detail"
	menuChoice     int
	scanProgress   float64
	scanMessage    string
	spinner        spinner.Model
	progress       progress.Model
	table          table.Model
	results        map[string]*ScanResult
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
	detailItems     []FileItem
	detailChoice    int
	detailOffset    int // Scroll offset for detail view
	// Scanning view fields
	scanningPaths   []string // Recently scanned paths
	scanFoundItems  int      // Number of items found
	scanTotalSize   int64    // Total size found so far
}

// Messages
type scanCompleteMsg struct {
	results   map[string]*ScanResult
	totalSize int64
}

type scanProgressMsg struct {
	percent float64
	message string
	path    string
	size    int64
	found   int
}

type cleanCompleteMsg struct {
	freed int64
	path  string // Path of the cleaned item
}

type diskUsageMsg struct {
	table table.Model
}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

// Initialize the model
func initialModel() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		scanner:  NewScanner(),
		state:    "menu",
		spinner:  s,
		progress: progress.New(progress.WithDefaultGradient()),
	}
}

// Init starts the spinner
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.state == "diskusage" {
				m.state = "menu"
				m.menuChoice = 0
				return m, nil
			}
			return m, tea.Quit

		case "enter":
			switch m.state {
			case "menu":
				switch m.menuChoice {
				case 0: // Full Scan
					m.state = "scanning"
					return m, tea.Batch(
						m.spinner.Tick,
						performScan(m.scanner),
					)
				case 1: // Dev Scan
					m.state = "scanning"
					m.scanMessage = "Starting Dev Scan - Deep scanning all projects..."
					m.scanningPaths = []string{}
					m.scanFoundItems = 0
					m.scanTotalSize = 0
					return m, tea.Batch(
						m.spinner.Tick,
						performDevScan(m.scanner),
					)
				case 2: // Quick Clean
					m.state = "scanning"
					return m, tea.Batch(
						m.spinner.Tick,
						performScan(m.scanner),
					)
				case 3: // Disk Usage
					return m, showDiskUsage()
				case 4: // Exit
					return m, tea.Quit
				}
			case "results":
				if m.menuChoice == len(m.results) {
					// Back to menu
					m.state = "menu"
					m.menuChoice = 0
				} else {
					// Enter detail view for the selected category
					categories := getSortedCategories(m.results)
					if m.menuChoice < len(categories) {
						category := categories[m.menuChoice]
						m.currentCategory = category
						m.currentPath = []string{category}
						m.detailItems = m.results[category].Items
						m.detailChoice = 0
						m.detailOffset = 0
						m.state = "detail"
					}
				}
			case "detail":
				if m.detailChoice < len(m.detailItems) {
					item := m.detailItems[m.detailChoice]
					if item.IsDir {
						// Explore subdirectory
						return m, exploreDirectory(&m, item.Path)
					}
				}
			}

		case "up", "k":
			if m.state == "menu" {
				if m.menuChoice > 0 {
					m.menuChoice--
				}
			} else if m.state == "results" {
				if m.menuChoice > 0 {
					m.menuChoice--
				}
			} else if m.state == "diskusage" {
				var cmd tea.Cmd
				m.diskUsageTable, cmd = m.diskUsageTable.Update(msg)
				return m, cmd
			} else if m.state == "detail" {
				if m.detailChoice > 0 {
					m.detailChoice--
					// Adjust viewport if needed
					if m.detailChoice < m.detailOffset {
						m.detailOffset = m.detailChoice
					}
				}
			}

		case "down", "j":
			if m.state == "menu" {
				if m.menuChoice < 4 {
					m.menuChoice++
				}
			} else if m.state == "results" {
				if m.menuChoice < len(m.results) {
					m.menuChoice++
				}
			} else if m.state == "diskusage" {
				var cmd tea.Cmd
				m.diskUsageTable, cmd = m.diskUsageTable.Update(msg)
				return m, cmd
			} else if m.state == "detail" {
				if m.detailChoice < len(m.detailItems)-1 {
					m.detailChoice++
					// Adjust viewport if needed
					viewportHeight := m.height - 15 // Account for header and footer
					if m.detailChoice >= m.detailOffset+viewportHeight {
						m.detailOffset = m.detailChoice - viewportHeight + 1
					}
				}
			}
			
		case "pgup":
			if m.state == "detail" {
				viewportHeight := m.height - 15
				m.detailChoice = max(0, m.detailChoice-viewportHeight)
				m.detailOffset = max(0, m.detailOffset-viewportHeight)
			}
			
		case "pgdown":
			if m.state == "detail" {
				viewportHeight := m.height - 15
				maxChoice := len(m.detailItems) - 1
				m.detailChoice = min(maxChoice, m.detailChoice+viewportHeight)
				maxOffset := max(0, len(m.detailItems)-viewportHeight)
				m.detailOffset = min(maxOffset, m.detailOffset+viewportHeight)
			}

		case "backspace", "delete":
			if m.state == "detail" && len(m.currentPath) > 1 {
				// Go back one level in detail view
				m.currentPath = m.currentPath[:len(m.currentPath)-1]
				if len(m.currentPath) == 1 {
					// Back to category root
					m.detailItems = m.results[m.currentCategory].Items
				} else {
					// Reload parent directory
					return m, exploreDirectory(&m, filepath.Dir(m.detailItems[0].Path))
				}
				m.detailChoice = 0
				m.detailOffset = 0
			}

		case "esc":
			if m.state == "detail" {
				m.state = "results"
				m.detailChoice = 0
			} else if m.state == "results" || m.state == "cleaning" || m.state == "diskusage" {
				m.state = "menu"
				m.menuChoice = 0
			}
			
		case "c":
			// Clean selected item in detail view
			if m.state == "detail" && m.detailChoice < len(m.detailItems) {
				item := m.detailItems[m.detailChoice]
				m.state = "cleaning"
				m.scanMessage = fmt.Sprintf("Cleaning %s...", item.Name)
				return m, performCleanItem(m.scanner, item)
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case scanProgressMsg:
		m.scanProgress = msg.percent
		m.scanMessage = msg.message
		if msg.path != "" {
			// Add to scanning paths (keep last 10)
			m.scanningPaths = append(m.scanningPaths, msg.path)
			if len(m.scanningPaths) > 10 {
				m.scanningPaths = m.scanningPaths[len(m.scanningPaths)-10:]
			}
			m.scanFoundItems = msg.found
			m.scanTotalSize += msg.size
		}
		return m, nil

	case scanCompleteMsg:
		m.results = msg.results
		m.totalSize = msg.totalSize
		m.state = "results"
		m.menuChoice = 0
		return m, nil

	case cleanCompleteMsg:
		if m.state == "cleaning" {
			// If we were in detail view, refresh it
			if msg.path != "" {
				// Remove the deleted item from the list
				newItems := []FileItem{}
				for _, item := range m.detailItems {
					if item.Path != msg.path {
						newItems = append(newItems, item)
					}
				}
				m.detailItems = newItems
				
				// Adjust selection if needed
				if m.detailChoice >= len(m.detailItems) && len(m.detailItems) > 0 {
					m.detailChoice = len(m.detailItems) - 1
				}
				if m.detailChoice < 0 {
					m.detailChoice = 0
				}
				
				// Adjust scroll offset if needed
				if m.detailOffset > 0 && m.detailChoice < m.detailOffset {
					m.detailOffset = m.detailChoice
				}
				
				// Update the total size in results if this was from a category
				if m.currentCategory != "" {
					if result, exists := m.results[m.currentCategory]; exists {
						// Update the category's items list
						newCategoryItems := []FileItem{}
						for _, item := range result.Items {
							if item.Path != msg.path {
								newCategoryItems = append(newCategoryItems, item)
							}
						}
						result.Items = newCategoryItems
						result.Total -= msg.freed
					}
				}
				
				m.totalSize -= msg.freed
				m.state = "detail" // Return to detail view
				
				// Show success message briefly
				deletedName := filepath.Base(msg.path)
				m.scanMessage = fmt.Sprintf("✅ Deleted %s (%s)", deletedName, humanize.Bytes(uint64(msg.freed)))
			} else {
				// Regular cleaning from results view
				m.totalSize -= msg.freed
				m.state = "results"
			}
		}
		return m, nil

	case diskUsageMsg:
		m.diskUsageTable = msg.table
		m.state = "diskusage"
		return m, nil

	case errMsg:
		m.err = msg
		return m, nil
	}

	return m, nil
}

// View renders the UI
func (m Model) View() string {
	var s strings.Builder

	// Header with padding
	header := titleStyle.Render("🧹 Mac Storage Cleaner")
	s.WriteString("\n")
	s.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, header))
	s.WriteString("\n\n\n")

	// Content with padding
	var content string
	switch m.state {
	case "menu":
		content = m.renderMenu()
	case "scanning":
		content = m.renderScanning()
	case "results":
		content = m.renderResults()
	case "cleaning":
		content = m.renderCleaning()
	case "diskusage":
		content = m.renderDiskUsage()
	case "detail":
		content = m.renderDetail()
	}

	// Add horizontal padding
	paddedContent := lipgloss.NewStyle().Padding(0, 3).Render(content)
	s.WriteString(paddedContent)

	if m.err != nil {
		s.WriteString("\n\n")
		errMsg := lipgloss.NewStyle().Padding(0, 3).Render(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		s.WriteString(errMsg)
	}

	s.WriteString("\n\n")
	return s.String()
}

func (m Model) renderMenu() string {
	var s strings.Builder

	items := []string{
		"🔍 Full System Scan",
		"💻 Dev Scan (Development caches & artifacts)",
		"🚀 Quick Clean (Safe files only)",
		"📊 Disk Usage Report",
		"❌ Exit",
	}

	s.WriteString(headerStyle.Render("Main Menu"))
	s.WriteString("\n\n\n")

	for i, item := range items {
		cursor := "  "
		style := lipgloss.NewStyle()

		if m.menuChoice == i {
			cursor = "▸ "
			style = selectedStyle
		}

		s.WriteString("  " + cursor + style.Render(item) + "\n\n")
	}

	s.WriteString("\n\n")
	s.WriteString(dimStyle.Render("Use ↑/↓ or j/k to navigate, Enter to select, q to quit"))

	return s.String()
}

func (m Model) renderScanning() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Scanning System..."))
	s.WriteString("\n\n")
	
	// Show scanning stats
	if m.scanFoundItems > 0 {
		stats := fmt.Sprintf("🔍 Found %d items | %s total",
			m.scanFoundItems,
			humanize.Bytes(uint64(m.scanTotalSize)))
		s.WriteString("  " + successStyle.Render(stats))
		s.WriteString("\n\n")
	}
	
	s.WriteString("  " + m.spinner.View() + " " + m.scanMessage)
	s.WriteString("\n\n")
	
	// Show recently scanned paths
	if len(m.scanningPaths) > 0 {
		s.WriteString("  " + dimStyle.Render("📁 Recently found:"))
		s.WriteString("\n")
		for _, path := range m.scanningPaths {
			// Truncate long paths
			displayPath := path
			if len(path) > 60 {
				displayPath = "..." + path[len(path)-57:]
			}
			s.WriteString("     " + dimStyle.Render(displayPath))
			s.WriteString("\n")
		}
		s.WriteString("\n")
	}
	
	// Show what we're looking for
	if m.state == "scanning" && strings.Contains(m.scanMessage, "Dev") {
		s.WriteString("  " + dimStyle.Render("🎯 Searching for:"))
		s.WriteString("\n")
		s.WriteString("  " + dimStyle.Render("   • node_modules, venv, __pycache__"))
		s.WriteString("\n")
		s.WriteString("  " + dimStyle.Render("   • build/dist folders, target directories"))
		s.WriteString("\n")
		s.WriteString("  " + dimStyle.Render("   • Package manager caches"))
		s.WriteString("\n\n")
	}
	
	s.WriteString(dimStyle.Render("Please wait, scanning your directories..."))

	return s.String()
}

func (m Model) renderResults() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Scan Results"))
	s.WriteString("\n\n\n")

	if len(m.results) == 0 {
		s.WriteString("  " + warningStyle.Render("No cleanable files found"))
		return s.String()
	}

	// Create table
	categories := getSortedCategories(m.results)

	s.WriteString("  Category                    Items        Size\n")
	s.WriteString("  ─────────────────────────────────────────────\n")

	for i, category := range categories {
		result := m.results[category]
		cursor := "  "
		style := lipgloss.NewStyle()

		if m.menuChoice == i {
			cursor = "▸ "
			style = selectedStyle
		}

		line := fmt.Sprintf("%-25s %5d  %10s",
			category,
			len(result.Items),
			humanize.Bytes(uint64(result.Total)),
		)

		s.WriteString("  " + cursor + style.Render(line) + "\n")
	}

	s.WriteString("  ─────────────────────────────────────────────\n")

	// Total
	totalLine := fmt.Sprintf("%-25s %5d  %10s",
		"TOTAL",
		m.getTotalItems(),
		humanize.Bytes(uint64(m.totalSize)),
	)
	s.WriteString("    " + successStyle.Render(totalLine) + "\n\n")

	// Back option
	cursor := "  "
	style := lipgloss.NewStyle()
	if m.menuChoice == len(categories) {
		cursor = "▸ "
		style = selectedStyle
	}
	s.WriteString("  " + cursor + style.Render("← Back to Menu") + "\n")

	s.WriteString("\n\n")
	s.WriteString(dimStyle.Render("Press Enter to explore category • ESC to go back to menu"))

	return s.String()
}

func (m Model) renderCleaning() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Cleaning Files..."))
	s.WriteString("\n\n\n")
	if m.scanMessage != "" {
		s.WriteString("  " + m.spinner.View() + " " + m.scanMessage)
	} else {
		s.WriteString("  " + m.spinner.View() + " Removing selected files...")
	}
	s.WriteString("\n\n\n")
	s.WriteString(m.progress.ViewAs(m.cleanProgress))

	return s.String()
}

func (m Model) renderDiskUsage() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Disk Usage Report"))
	s.WriteString("\n\n\n")
	s.WriteString(m.diskUsageTable.View())
	s.WriteString("\n\n\n")
	s.WriteString(dimStyle.Render("Use ↑/↓ or j/k to navigate, ESC or q to go back to menu"))

	return s.String()
}

func (m Model) renderDetail() string {
	var s strings.Builder

	// Breadcrumb navigation
	breadcrumb := strings.Join(m.currentPath, " > ")
	s.WriteString(headerStyle.Render("📁 " + breadcrumb))
	s.WriteString("\n\n")
	
	// Show success message if item was just cleaned
	if m.state == "detail" && strings.Contains(m.scanMessage, "✅") {
		s.WriteString("  " + successStyle.Render(m.scanMessage))
		s.WriteString("\n")
	}
	s.WriteString("\n")

	if len(m.detailItems) == 0 {
		s.WriteString("  " + dimStyle.Render("No items found"))
		s.WriteString("\n\n")
		s.WriteString(dimStyle.Render("Press Backspace to go back"))
		return s.String()
	}

	// Calculate viewport
	viewportHeight := m.height - 15 // Account for header, footer, and padding
	if viewportHeight < 5 {
		viewportHeight = 5 // Minimum viewport height
	}

	// Determine visible range
	startIdx := m.detailOffset
	endIdx := min(startIdx+viewportHeight, len(m.detailItems))

	// Show scroll indicator if needed
	if len(m.detailItems) > viewportHeight {
		scrollInfo := fmt.Sprintf("[%d-%d of %d items]", startIdx+1, endIdx, len(m.detailItems))
		s.WriteString("  " + dimStyle.Render(scrollInfo))
		if startIdx > 0 {
			s.WriteString(dimStyle.Render(" ↑"))
		}
		if endIdx < len(m.detailItems) {
			s.WriteString(dimStyle.Render(" ↓"))
		}
		s.WriteString("\n\n")
	}

	// Display visible items
	for i := startIdx; i < endIdx; i++ {
		item := m.detailItems[i]
		cursor := "  "
		style := lipgloss.NewStyle()

		if m.detailChoice == i {
			cursor = "▸ "
			style = selectedStyle
		}

		icon := "📄"
		if item.IsDir {
			icon = "📁"
		}

		// Adjust name width based on terminal width
		nameWidth := min(50, m.width-30)
		line := fmt.Sprintf("%s %-*s %10s",
			icon,
			nameWidth,
			truncatePath(item.Name, nameWidth),
			humanize.Bytes(uint64(item.Size)),
		)

		s.WriteString("  " + cursor + style.Render(line) + "\n")
	}

	// Fill remaining viewport space
	remainingLines := viewportHeight - (endIdx - startIdx)
	for i := 0; i < remainingLines && i < 3; i++ {
		s.WriteString("\n")
	}
	
	// Show total size for current view
	var totalSize int64
	for _, item := range m.detailItems {
		totalSize += item.Size
	}
	s.WriteString("\n")
	s.WriteString("  " + dimStyle.Render(fmt.Sprintf("Total: %s", humanize.Bytes(uint64(totalSize)))))
	s.WriteString("\n\n")
	
	// Instructions
	s.WriteString(dimStyle.Render("↑/↓ Navigate • PgUp/PgDn: Page • Enter: Open • Backspace: Up • c: Clean • ESC: Back"))

	return s.String()
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return path[:maxLen-3] + "..."
}

func (m Model) getTotalItems() int {
	total := 0
	for _, result := range m.results {
		total += len(result.Items)
	}
	return total
}

// Scanner implementation
func NewScanner() *Scanner {
	homeDir, _ := os.UserHomeDir()
	return &Scanner{
		HomeDir: homeDir,
		Results: make(map[string]*ScanResult),
	}
}

func (s *Scanner) getDirSize(path string) (int64, error) {
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

func (s *Scanner) scanCacheFiles() *ScanResult {
	result := &ScanResult{
		Category: "Cache Files",
		Items:    []FileItem{},
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
			size, _ := s.getDirSize(path)
			if size > 0 {
				result.Items = append(result.Items, FileItem{
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

func (s *Scanner) scanLogFiles() *ScanResult {
	result := &ScanResult{
		Category: "Log Files",
		Items:    []FileItem{},
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
					result.Items = append(result.Items, FileItem{
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

func (s *Scanner) scanTrash() *ScanResult {
	result := &ScanResult{
		Category: "Trash",
		Items:    []FileItem{},
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
		size, _ := s.getDirSize(path)
		result.Items = append(result.Items, FileItem{
			Path: path,
			Size: size,
			Name: entry.Name(),
		})
		result.Total += size
	}

	return result
}

func (s *Scanner) scanDownloads() *ScanResult {
	result := &ScanResult{
		Category: "Old Downloads",
		Items:    []FileItem{},
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
			size, _ := s.getDirSize(path)
			age := int(time.Since(info.ModTime()).Hours() / 24)

			result.Items = append(result.Items, FileItem{
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

func (s *Scanner) scanXcodeFiles() *ScanResult {
	result := &ScanResult{
		Category: "Xcode Files",
		Items:    []FileItem{},
	}

	xcodeDirs := []string{
		filepath.Join(s.HomeDir, "Library", "Developer", "Xcode", "DerivedData"),
		filepath.Join(s.HomeDir, "Library", "Developer", "Xcode", "Archives"),
		filepath.Join(s.HomeDir, "Library", "Developer", "CoreSimulator", "Devices"),
	}

	for _, dir := range xcodeDirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			size, _ := s.getDirSize(path)
			if size > 0 {
				result.Items = append(result.Items, FileItem{
					Path: path,
					Size: size,
					Name: "Xcode: " + entry.Name(),
				})
				result.Total += size
			}
		}
	}

	return result
}

func (s *Scanner) scanBrewCache() *ScanResult {
	result := &ScanResult{
		Category: "Homebrew Cache",
		Items:    []FileItem{},
	}

	brewCache := filepath.Join(s.HomeDir, "Library", "Caches", "Homebrew")
	if _, err := os.Stat(brewCache); err != nil {
		return result
	}

	entries, err := os.ReadDir(brewCache)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		path := filepath.Join(brewCache, entry.Name())
		size, _ := s.getDirSize(path)
		result.Items = append(result.Items, FileItem{
			Path: path,
			Size: size,
			Name: "Brew: " + entry.Name(),
		})
		result.Total += size
	}

	return result
}

func (s *Scanner) scanNodeModulesWithProgress(progressChan chan<- scanProgressMsg) *ScanResult {
	result := &ScanResult{
		Category: "Node Modules",
		Items:    []FileItem{},
	}

	itemCount := 0
	// Deep scan entire home directory
	filepath.WalkDir(s.HomeDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			// Skip system directories
			if strings.Contains(path, "/Library/") && !strings.Contains(path, "/Documents/") ||
			   strings.Contains(path, "/System/") ||
			   strings.Contains(path, "/.Trash/") ||
			   strings.Contains(path, "/Applications/") && !strings.Contains(path, "/Documents/") {
				return filepath.SkipDir
			}

			if d.Name() == "node_modules" {
				size, _ := s.getDirSize(path)
				if size > 0 {
					// Get project path for better context
					projectPath := filepath.Dir(path)
					relPath, _ := filepath.Rel(s.HomeDir, projectPath)
					item := FileItem{
						Path:  path,
						Size:  size,
						Name:  fmt.Sprintf("📦 %s", relPath),
						IsDir: true,
					}
					result.Items = append(result.Items, item)
					result.Total += size
					itemCount++
					
					// Send progress update
					if progressChan != nil {
						select {
						case progressChan <- scanProgressMsg{
							path:  relPath,
							size:  size,
							found: itemCount,
							message: fmt.Sprintf("Found node_modules in %s", relPath),
						}:
						default:
						}
					}
				}
				return filepath.SkipDir
			}
		}

		return nil
	})

	return result
}

func (s *Scanner) scanNodeModules() *ScanResult {
	result := &ScanResult{
		Category: "Node Modules",
		Items:    []FileItem{},
	}

	// Deep scan entire home directory
	filepath.WalkDir(s.HomeDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			// Skip system directories
			if strings.Contains(path, "/Library/") && !strings.Contains(path, "/Documents/") ||
			   strings.Contains(path, "/System/") ||
			   strings.Contains(path, "/.Trash/") ||
			   strings.Contains(path, "/Applications/") && !strings.Contains(path, "/Documents/") {
				return filepath.SkipDir
			}

			if d.Name() == "node_modules" {
				size, _ := s.getDirSize(path)
				if size > 0 {
					// Get project path for better context
					projectPath := filepath.Dir(path)
					relPath, _ := filepath.Rel(s.HomeDir, projectPath)
					result.Items = append(result.Items, FileItem{
						Path:  path,
						Size:  size,
						Name:  fmt.Sprintf("📦 %s", relPath),
						IsDir: true,
					})
					result.Total += size
				}
				return filepath.SkipDir
			}
		}

		return nil
	})

	return result
}

// Development-specific scanners
func (s *Scanner) scanPythonArtifactsWithProgress(progressChan chan<- scanProgressMsg) *ScanResult {
	result := s.scanPythonArtifacts()
	// Send found items as progress
	for _, item := range result.Items {
		if progressChan != nil {
			select {
			case progressChan <- scanProgressMsg{
				path:    item.Name,
				size:    item.Size,
				found:   len(result.Items),
				message: "Scanning Python artifacts...",
			}:
			default:
			}
		}
	}
	return result
}

func (s *Scanner) scanRustArtifactsWithProgress(progressChan chan<- scanProgressMsg) *ScanResult {
	result := s.scanRustArtifacts()
	for _, item := range result.Items {
		if progressChan != nil {
			select {
			case progressChan <- scanProgressMsg{
				path:    item.Name,
				size:    item.Size,
				found:   len(result.Items),
				message: "Scanning Rust artifacts...",
			}:
			default:
			}
		}
	}
	return result
}

func (s *Scanner) scanBuildArtifactsWithProgress(progressChan chan<- scanProgressMsg) *ScanResult {
	result := s.scanBuildArtifacts()
	for _, item := range result.Items {
		if progressChan != nil {
			select {
			case progressChan <- scanProgressMsg{
				path:    item.Name,
				size:    item.Size,
				found:   len(result.Items),
				message: "Scanning build artifacts...",
			}:
			default:
			}
		}
	}
	return result
}

func (s *Scanner) scanPythonArtifacts() *ScanResult {
	result := &ScanResult{
		Category: "Python Artifacts",
		Items:    []FileItem{},
	}

	// Python cache directories
	pythonCaches := []string{
		filepath.Join(s.HomeDir, ".cache", "pip"),
		filepath.Join(s.HomeDir, "Library", "Caches", "pip"),
		filepath.Join(s.HomeDir, ".conda", "pkgs"),
	}

	// Add Python cache directories
	for _, dir := range pythonCaches {
		if _, err := os.Stat(dir); err == nil {
			size, _ := s.getDirSize(dir)
			if size > 0 {
				result.Items = append(result.Items, FileItem{
					Path: dir,
					Size: size,
					Name: "Python: " + filepath.Base(dir) + " cache",
				})
				result.Total += size
			}
		}
	}

	// Deep scan for Python virtual environments and caches
	filepath.WalkDir(s.HomeDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			// Skip system directories
			if strings.Contains(path, "/Library/") && !strings.Contains(path, "/Documents/") ||
			   strings.Contains(path, "/System/") ||
			   strings.Contains(path, "/.Trash/") ||
			   strings.Contains(path, "/Applications/") && !strings.Contains(path, "/Documents/") {
				return filepath.SkipDir
			}

			name := d.Name()
			if name == "__pycache__" || name == "venv" || name == ".venv" || 
			   name == "env" || name == ".env" || name == "virtualenv" || 
			   name == ".pytest_cache" || name == ".tox" || name == ".mypy_cache" {
				size, _ := s.getDirSize(path)
				if size > 0 {
					projectPath := filepath.Dir(path)
					relPath, _ := filepath.Rel(s.HomeDir, projectPath)
					result.Items = append(result.Items, FileItem{
						Path:  path,
						Size:  size,
						Name:  fmt.Sprintf("🐍 %s (%s)", relPath, name),
						IsDir: true,
					})
					result.Total += size
				}
				return filepath.SkipDir
			}
		}

		return nil
	})

	return result
}

func (s *Scanner) scanGoArtifacts() *ScanResult {
	result := &ScanResult{
		Category: "Go Artifacts",
		Items:    []FileItem{},
	}

	// Go module cache
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		goPath = filepath.Join(s.HomeDir, "go")
	}

	goCaches := []string{
		filepath.Join(goPath, "pkg", "mod"),
		filepath.Join(s.HomeDir, ".cache", "go-build"),
		filepath.Join(s.HomeDir, "Library", "Caches", "go-build"),
	}

	for _, dir := range goCaches {
		if _, err := os.Stat(dir); err == nil {
			size, _ := s.getDirSize(dir)
			if size > 0 {
				result.Items = append(result.Items, FileItem{
					Path: dir,
					Size: size,
					Name: "Go: " + filepath.Base(dir),
				})
				result.Total += size
			}
		}
	}

	return result
}

func (s *Scanner) scanRustArtifacts() *ScanResult {
	result := &ScanResult{
		Category: "Rust Artifacts",
		Items:    []FileItem{},
	}

	// Cargo cache
	cargoHome := os.Getenv("CARGO_HOME")
	if cargoHome == "" {
		cargoHome = filepath.Join(s.HomeDir, ".cargo")
	}

	registryCache := filepath.Join(cargoHome, "registry", "cache")
	if _, err := os.Stat(registryCache); err == nil {
		size, _ := s.getDirSize(registryCache)
		if size > 0 {
			result.Items = append(result.Items, FileItem{
				Path:  registryCache,
				Size:  size,
				Name:  "🦀 Cargo registry cache",
				IsDir: true,
			})
			result.Total += size
		}
	}

	// Deep scan for Rust target directories
	filepath.WalkDir(s.HomeDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			// Skip system directories
			if strings.Contains(path, "/Library/") && !strings.Contains(path, "/Documents/") ||
			   strings.Contains(path, "/System/") ||
			   strings.Contains(path, "/.Trash/") ||
			   strings.Contains(path, "/Applications/") && !strings.Contains(path, "/Documents/") {
				return filepath.SkipDir
			}

			if d.Name() == "target" {
				// Check if it's a Rust project (has Cargo.toml in parent)
				if _, err := os.Stat(filepath.Join(filepath.Dir(path), "Cargo.toml")); err == nil {
					size, _ := s.getDirSize(path)
					if size > 0 {
						projectPath := filepath.Dir(path)
						relPath, _ := filepath.Rel(s.HomeDir, projectPath)
						result.Items = append(result.Items, FileItem{
							Path:  path,
							Size:  size,
							Name:  fmt.Sprintf("🦀 %s", relPath),
							IsDir: true,
						})
						result.Total += size
					}
					return filepath.SkipDir
				}
			}
		}

		return nil
	})

	return result
}

func (s *Scanner) scanDockerArtifacts() *ScanResult {
	result := &ScanResult{
		Category: "Docker Artifacts",
		Items:    []FileItem{},
	}

	// Docker Desktop data
	dockerData := filepath.Join(s.HomeDir, "Library", "Containers", "com.docker.docker", "Data")
	if _, err := os.Stat(dockerData); err == nil {
		size, _ := s.getDirSize(dockerData)
		if size > 100*1024*1024 { // Only if > 100MB
			result.Items = append(result.Items, FileItem{
				Path: dockerData,
				Size: size,
				Name: "Docker: Desktop Data",
			})
			result.Total += size
		}
	}

	return result
}

func (s *Scanner) scanIDECaches() *ScanResult {
	result := &ScanResult{
		Category: "IDE Caches",
		Items:    []FileItem{},
	}

	// VS Code
	vscodeDirs := []string{
		filepath.Join(s.HomeDir, "Library", "Application Support", "Code", "Cache"),
		filepath.Join(s.HomeDir, "Library", "Application Support", "Code", "CachedData"),
		filepath.Join(s.HomeDir, ".vscode", "extensions"),
	}

	for _, dir := range vscodeDirs {
		if _, err := os.Stat(dir); err == nil {
			size, _ := s.getDirSize(dir)
			if size > 0 {
				result.Items = append(result.Items, FileItem{
					Path: dir,
					Size: size,
					Name: "VS Code: " + filepath.Base(dir),
				})
				result.Total += size
			}
		}
	}

	// JetBrains IDEs
	jetbrainsDirs := []string{
		filepath.Join(s.HomeDir, "Library", "Caches", "JetBrains"),
		filepath.Join(s.HomeDir, "Library", "Application Support", "JetBrains"),
	}

	for _, dir := range jetbrainsDirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				path := filepath.Join(dir, entry.Name())
				size, _ := s.getDirSize(path)
				if size > 0 {
					result.Items = append(result.Items, FileItem{
						Path: path,
						Size: size,
						Name: "JetBrains: " + entry.Name(),
					})
					result.Total += size
				}
			}
		}
	}

	return result
}

func (s *Scanner) scanJavaArtifacts() *ScanResult {
	result := &ScanResult{
		Category: "Java/JVM Artifacts",
		Items:    []FileItem{},
	}

	// Maven cache
	m2Repo := filepath.Join(s.HomeDir, ".m2", "repository")
	if _, err := os.Stat(m2Repo); err == nil {
		size, _ := s.getDirSize(m2Repo)
		if size > 0 {
			result.Items = append(result.Items, FileItem{
				Path: m2Repo,
				Size: size,
				Name: "Maven: .m2 repository",
			})
			result.Total += size
		}
	}

	// Gradle cache
	gradleCache := filepath.Join(s.HomeDir, ".gradle", "caches")
	if _, err := os.Stat(gradleCache); err == nil {
		size, _ := s.getDirSize(gradleCache)
		if size > 0 {
			result.Items = append(result.Items, FileItem{
				Path: gradleCache,
				Size: size,
				Name: "Gradle: caches",
			})
			result.Total += size
		}
	}

	return result
}

func (s *Scanner) scanNpmYarnCaches() *ScanResult {
	result := &ScanResult{
		Category: "NPM/Yarn/PNPM Caches",
		Items:    []FileItem{},
	}

	nodeCaches := []struct {
		path string
		name string
	}{
		{filepath.Join(s.HomeDir, ".npm"), "NPM cache"},
		{filepath.Join(s.HomeDir, "Library", "Caches", "npm"), "NPM cache (Library)"},
		{filepath.Join(s.HomeDir, ".yarn", "cache"), "Yarn cache"},
		{filepath.Join(s.HomeDir, "Library", "Caches", "Yarn"), "Yarn cache (Library)"},
		{filepath.Join(s.HomeDir, ".pnpm-store"), "PNPM store"},
	}

	for _, cache := range nodeCaches {
		if _, err := os.Stat(cache.path); err == nil {
			size, _ := s.getDirSize(cache.path)
			if size > 0 {
				result.Items = append(result.Items, FileItem{
					Path: cache.path,
					Size: size,
					Name: cache.name,
				})
				result.Total += size
			}
		}
	}

	return result
}

func (s *Scanner) scanRubyArtifacts() *ScanResult {
	result := &ScanResult{
		Category: "Ruby Artifacts",
		Items:    []FileItem{},
	}

	// Ruby gems
	gemHome := os.Getenv("GEM_HOME")
	if gemHome == "" {
		gemHome = filepath.Join(s.HomeDir, ".gem")
	}

	if _, err := os.Stat(gemHome); err == nil {
		size, _ := s.getDirSize(gemHome)
		if size > 0 {
			result.Items = append(result.Items, FileItem{
				Path: gemHome,
				Size: size,
				Name: "Ruby: Gem cache",
			})
			result.Total += size
		}
	}

	// Bundler
	bundleCache := filepath.Join(s.HomeDir, ".bundle", "cache")
	if _, err := os.Stat(bundleCache); err == nil {
		size, _ := s.getDirSize(bundleCache)
		if size > 0 {
			result.Items = append(result.Items, FileItem{
				Path: bundleCache,
				Size: size,
				Name: "Ruby: Bundler cache",
			})
			result.Total += size
		}
	}

	return result
}

func (s *Scanner) scanCocoaPods() *ScanResult {
	result := &ScanResult{
		Category: "CocoaPods",
		Items:    []FileItem{},
	}

	cocoapodsCache := filepath.Join(s.HomeDir, "Library", "Caches", "CocoaPods")
	if _, err := os.Stat(cocoapodsCache); err == nil {
		size, _ := s.getDirSize(cocoapodsCache)
		if size > 0 {
			result.Items = append(result.Items, FileItem{
				Path: cocoapodsCache,
				Size: size,
				Name: "CocoaPods cache",
			})
			result.Total += size
		}
	}

	return result
}

// Additional deep scan function for build artifacts
func (s *Scanner) scanBuildArtifacts() *ScanResult {
	result := &ScanResult{
		Category: "Build Artifacts",
		Items:    []FileItem{},
	}

	// Deep scan for various build directories
	filepath.WalkDir(s.HomeDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			// Skip system directories
			if strings.Contains(path, "/Library/") && !strings.Contains(path, "/Documents/") ||
			   strings.Contains(path, "/System/") ||
			   strings.Contains(path, "/.Trash/") ||
			   strings.Contains(path, "/Applications/") && !strings.Contains(path, "/Documents/") {
				return filepath.SkipDir
			}

			name := d.Name()
			// Check for common build directories
			if name == "dist" || name == "build" || name == "out" || 
			   name == ".next" || name == ".nuxt" || name == ".output" ||
			   name == "coverage" || name == ".nyc_output" || name == ".parcel-cache" ||
			   name == "tmp" || name == "temp" {
				// Check if it's likely a project build dir (has package.json, Cargo.toml, etc. in parent)
				parentDir := filepath.Dir(path)
				isProjectDir := false
				
				projectFiles := []string{"package.json", "Cargo.toml", "pom.xml", "build.gradle", "Makefile", "CMakeLists.txt"}
				for _, pf := range projectFiles {
					if _, err := os.Stat(filepath.Join(parentDir, pf)); err == nil {
						isProjectDir = true
						break
					}
				}
				
				if isProjectDir {
					size, _ := s.getDirSize(path)
					if size > 0 {
						relPath, _ := filepath.Rel(s.HomeDir, parentDir)
						result.Items = append(result.Items, FileItem{
							Path:  path,
							Size:  size,
							Name:  fmt.Sprintf("🔨 %s (%s)", relPath, name),
							IsDir: true,
						})
						result.Total += size
					}
					return filepath.SkipDir
				}
			}
		}

		return nil
	})

	return result
}

// Channel for sending scan updates
var scanUpdateChan chan scanProgressMsg

// Command functions
func performDevScan(scanner *Scanner) tea.Cmd {
	return func() tea.Msg {
		// Create a channel for live updates
		scanUpdateChan = make(chan scanProgressMsg, 100)
		
		// Start a goroutine to send updates
		go func() {
			for _ = range scanUpdateChan {
				// Updates are being handled by the channel
			}
		}()
		
		// Note: Deep scan operations run in parallel but may take longer
		// due to traversing entire home directory
		scanners := []struct {
			name string
			fn   func(chan<- scanProgressMsg) *ScanResult
		}{
			{"Node Modules", scanner.scanNodeModulesWithProgress},
			{"Python Artifacts", scanner.scanPythonArtifactsWithProgress},
			{"Rust Artifacts", scanner.scanRustArtifactsWithProgress},
			{"Build Artifacts", scanner.scanBuildArtifactsWithProgress},
			{"NPM/Yarn/PNPM Caches", func(ch chan<- scanProgressMsg) *ScanResult { return scanner.scanNpmYarnCaches() }},
			{"Go Artifacts", func(ch chan<- scanProgressMsg) *ScanResult { return scanner.scanGoArtifacts() }},
			{"Java/JVM Artifacts", func(ch chan<- scanProgressMsg) *ScanResult { return scanner.scanJavaArtifacts() }},
			{"Ruby Artifacts", func(ch chan<- scanProgressMsg) *ScanResult { return scanner.scanRubyArtifacts() }},
			{"Docker Artifacts", func(ch chan<- scanProgressMsg) *ScanResult { return scanner.scanDockerArtifacts() }},
			{"IDE Caches", func(ch chan<- scanProgressMsg) *ScanResult { return scanner.scanIDECaches() }},
			{"Xcode Files", func(ch chan<- scanProgressMsg) *ScanResult { return scanner.scanXcodeFiles() }},
			{"Homebrew Cache", func(ch chan<- scanProgressMsg) *ScanResult { return scanner.scanBrewCache() }},
			{"CocoaPods", func(ch chan<- scanProgressMsg) *ScanResult { return scanner.scanCocoaPods() }},
		}

		results := make(map[string]*ScanResult)
		var totalSize int64
		var totalFound int

		// Use goroutines for parallel scanning
		var wg sync.WaitGroup
		for _, sc := range scanners {
			wg.Add(1)
			go func(name string, scanFunc func(chan<- scanProgressMsg) *ScanResult) {
				defer wg.Done()

				result := scanFunc(scanUpdateChan)
				if result.Total > 0 {
					scanner.mu.Lock()
					results[name] = result
					totalSize += result.Total
					totalFound += len(result.Items)
					scanner.mu.Unlock()
				}
			}(sc.name, sc.fn)
		}

		wg.Wait()
		close(scanUpdateChan)

		return scanCompleteMsg{
			results:   results,
			totalSize: totalSize,
		}
	}
}

func performScan(scanner *Scanner) tea.Cmd {
	return func() tea.Msg {
		scanners := []struct {
			name string
			fn   func() *ScanResult
		}{
			{"Cache Files", scanner.scanCacheFiles},
			{"Log Files", scanner.scanLogFiles},
			{"Trash", scanner.scanTrash},
			{"Old Downloads", scanner.scanDownloads},
			{"Xcode Files", scanner.scanXcodeFiles},
			{"Homebrew Cache", scanner.scanBrewCache},
			{"Node Modules", scanner.scanNodeModules},
		}

		results := make(map[string]*ScanResult)
		var totalSize int64
		var completed int32

		// Use goroutines for parallel scanning
		var wg sync.WaitGroup
		for _, sc := range scanners {
			wg.Add(1)
			go func(name string, scanFunc func() *ScanResult) {
				defer wg.Done()

				result := scanFunc()
				if result.Total > 0 {
					scanner.mu.Lock()
					results[name] = result
					totalSize += result.Total
					scanner.mu.Unlock()
				}

				atomic.AddInt32(&completed, 1)
			}(sc.name, sc.fn)
		}

		wg.Wait()

		return scanCompleteMsg{
			results:   results,
			totalSize: totalSize,
		}
	}
}

func performClean(scanner *Scanner, category string) tea.Cmd {
	return func() tea.Msg {
		result, exists := scanner.Results[category]
		if !exists {
			return cleanCompleteMsg{freed: 0}
		}

		var freed int64
		for _, item := range result.Items {
			err := os.RemoveAll(item.Path)
			if err == nil {
				freed += item.Size
			}
		}

		// Remove from results
		delete(scanner.Results, category)

		return cleanCompleteMsg{freed: freed, path: ""}
	}
}

func showDiskUsage() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("df", "-h")
		output, err := cmd.Output()
		if err != nil {
			return errMsg{err}
		}

		// Parse the disk usage output and create a table
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) == 0 {
			return errMsg{fmt.Errorf("no disk usage data")}
		}

		// Create table data
		var rows []table.Row
		for i, line := range lines {
			if i == 0 {
				continue // Skip header
			}

			fields := strings.Fields(line)
			if len(fields) < 9 {
				continue // Skip malformed lines
			}

			// Extract fields for macOS df output format
			filesystem := fields[0]
			size := fields[1]
			used := fields[2]
			avail := fields[3]
			capacity := fields[4]
			mountPoint := strings.Join(fields[8:], " ")

			// Truncate long filesystem names
			if len(filesystem) > 25 {
				filesystem = filesystem[:22] + "..."
			}

			rows = append(rows, table.Row{
				filesystem,
				size,
				used,
				avail,
				capacity,
				mountPoint,
			})
		}

		// Create table
		columns := []table.Column{
			{Title: "Filesystem", Width: 25},
			{Title: "Size", Width: 8},
			{Title: "Used", Width: 8},
			{Title: "Avail", Width: 8},
			{Title: "Capacity", Width: 10},
			{Title: "Mounted on", Width: 40},
		}

		t := table.New(
			table.WithColumns(columns),
			table.WithRows(rows),
			table.WithFocused(true),
			table.WithHeight(len(rows)+1),
		)

		s := table.DefaultStyles()
		s.Header = s.Header.
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			BorderBottom(true).
			Bold(false)
		s.Selected = s.Selected.
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			Bold(false)
		t.SetStyles(s)

		return diskUsageMsg{table: t}
	}
}

func getSortedCategories(results map[string]*ScanResult) []string {
	categories := make([]string, 0, len(results))
	for category := range results {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	return categories
}

// exploreDirectory loads the contents of a directory for detail view
func exploreDirectory(m *Model, dirPath string) tea.Cmd {
	return func() tea.Msg {
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			return errMsg{err}
		}

		var items []FileItem
		for _, entry := range entries {
			path := filepath.Join(dirPath, entry.Name())
			info, err := entry.Info()
			if err != nil {
				continue
			}

			size := info.Size()
			if entry.IsDir() {
				// Calculate directory size
				dirSize, _ := calculateDirSize(path)
				size = dirSize
			}

			items = append(items, FileItem{
				Path:  path,
				Name:  entry.Name(),
				Size:  size,
				IsDir: entry.IsDir(),
			})
		}

		// Sort by size descending
		sort.Slice(items, func(i, j int) bool {
			return items[i].Size > items[j].Size
		})

		// Update model
		m.detailItems = items
		m.currentPath = append(m.currentPath, filepath.Base(dirPath))
		m.detailChoice = 0
		m.detailOffset = 0

		return nil
	}
}

// calculateDirSize recursively calculates the size of a directory
func calculateDirSize(path string) (int64, error) {
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

// performCleanItem cleans a single item from detail view
func performCleanItem(scanner *Scanner, item FileItem) tea.Cmd {
	return func() tea.Msg {
		// First check if the item still exists
		if _, err := os.Stat(item.Path); os.IsNotExist(err) {
			// Item already deleted
			return cleanCompleteMsg{freed: 0, path: item.Path}
		}
		
		// Calculate actual size before deletion
		var actualSize int64
		if item.IsDir {
			actualSize, _ = calculateDirSize(item.Path)
		} else {
			if info, err := os.Stat(item.Path); err == nil {
				actualSize = info.Size()
			}
		}
		
		err := os.RemoveAll(item.Path)
		if err != nil {
			return errMsg{err}
		}
		
		return cleanCompleteMsg{freed: actualSize, path: item.Path}
	}
}

// Message type for updating detail items
type detailUpdateMsg struct {
	items []FileItem
	path  []string
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
