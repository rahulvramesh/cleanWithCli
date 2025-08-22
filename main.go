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
	Path string
	Size int64
	Name string
	Age  int // days old
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
	state          string // "menu", "scanning", "results", "cleaning", "diskusage"
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
}

// Messages
type scanCompleteMsg struct {
	results   map[string]*ScanResult
	totalSize int64
}

type scanProgressMsg struct {
	percent float64
	message string
}

type cleanCompleteMsg struct {
	freed int64
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
					// Start cleaning selected category
					m.state = "cleaning"
					categories := getSortedCategories(m.results)
					if m.menuChoice < len(categories) {
						category := categories[m.menuChoice]
						return m, performClean(m.scanner, category)
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
			}

		case "esc":
			if m.state == "results" || m.state == "cleaning" || m.state == "diskusage" {
				m.state = "menu"
				m.menuChoice = 0
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case scanProgressMsg:
		m.scanProgress = msg.percent
		m.scanMessage = msg.message
		return m, nil

	case scanCompleteMsg:
		m.results = msg.results
		m.totalSize = msg.totalSize
		m.state = "results"
		m.menuChoice = 0
		return m, nil

	case cleanCompleteMsg:
		// Remove cleaned items from results
		m.totalSize -= msg.freed
		m.state = "results"
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
	s.WriteString("\n\n\n")
	s.WriteString("  " + m.spinner.View() + " " + m.scanMessage)
	s.WriteString("\n\n\n")
	s.WriteString(m.progress.ViewAs(m.scanProgress))
	s.WriteString("\n\n\n")
	s.WriteString(dimStyle.Render("Please wait, this may take a moment..."))

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
	s.WriteString(dimStyle.Render("Select a category to clean or press Esc to go back"))

	return s.String()
}

func (m Model) renderCleaning() string {
	var s strings.Builder

	s.WriteString(headerStyle.Render("Cleaning Files..."))
	s.WriteString("\n\n\n")
	s.WriteString("  " + m.spinner.View() + " Removing selected files...")
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
					Path: path,
					Size: size,
					Name: entry.Name(),
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

func (s *Scanner) scanNodeModules() *ScanResult {
	result := &ScanResult{
		Category: "Node Modules",
		Items:    []FileItem{},
	}

	searchDirs := []string{
		filepath.Join(s.HomeDir, "Desktop"),
		filepath.Join(s.HomeDir, "Documents"),
		filepath.Join(s.HomeDir, "Developer"),
		filepath.Join(s.HomeDir, "Projects"),
	}

	for _, dir := range searchDirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			if d.IsDir() && d.Name() == "node_modules" {
				size, _ := s.getDirSize(path)
				if size > 0 {
					parentDir := filepath.Base(filepath.Dir(path))
					result.Items = append(result.Items, FileItem{
						Path: path,
						Size: size,
						Name: fmt.Sprintf("node_modules in %s", parentDir),
					})
					result.Total += size
				}
				return filepath.SkipDir // Don't traverse inside node_modules
			}

			// Skip hidden directories and common non-project directories
			if d.IsDir() && (strings.HasPrefix(d.Name(), ".") || d.Name() == "Library") {
				return filepath.SkipDir
			}

			return nil
		})
	}

	return result
}

// Development-specific scanners
func (s *Scanner) scanPythonArtifacts() *ScanResult {
	result := &ScanResult{
		Category: "Python Artifacts",
		Items:    []FileItem{},
	}

	searchDirs := []string{
		filepath.Join(s.HomeDir, "Desktop"),
		filepath.Join(s.HomeDir, "Documents"),
		filepath.Join(s.HomeDir, "Developer"),
		filepath.Join(s.HomeDir, "Projects"),
		filepath.Join(s.HomeDir, "Code"),
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

	// Search for __pycache__, venv, .env directories
	for _, dir := range searchDirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			if d.IsDir() {
				name := d.Name()
				if name == "__pycache__" || name == "venv" || name == ".venv" || 
				   name == "env" || name == ".env" || name == "virtualenv" {
					size, _ := s.getDirSize(path)
					if size > 0 {
						parentDir := filepath.Base(filepath.Dir(path))
						result.Items = append(result.Items, FileItem{
							Path: path,
							Size: size,
							Name: fmt.Sprintf("Python: %s in %s", name, parentDir),
						})
						result.Total += size
					}
					return filepath.SkipDir
				}

				// Skip hidden directories except .env/.venv
				if strings.HasPrefix(name, ".") && name != ".env" && name != ".venv" {
					return filepath.SkipDir
				}
			}

			return nil
		})
	}

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
				Path: registryCache,
				Size: size,
				Name: "Rust: Cargo registry cache",
			})
			result.Total += size
		}
	}

	// Search for target directories in Rust projects
	searchDirs := []string{
		filepath.Join(s.HomeDir, "Desktop"),
		filepath.Join(s.HomeDir, "Documents"),
		filepath.Join(s.HomeDir, "Developer"),
		filepath.Join(s.HomeDir, "Projects"),
	}

	for _, dir := range searchDirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			if d.IsDir() && d.Name() == "target" {
				// Check if it's a Rust project (has Cargo.toml in parent)
				if _, err := os.Stat(filepath.Join(filepath.Dir(path), "Cargo.toml")); err == nil {
					size, _ := s.getDirSize(path)
					if size > 0 {
						parentDir := filepath.Base(filepath.Dir(path))
						result.Items = append(result.Items, FileItem{
							Path: path,
							Size: size,
							Name: fmt.Sprintf("Rust: target in %s", parentDir),
						})
						result.Total += size
					}
					return filepath.SkipDir
				}
			}

			if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}

			return nil
		})
	}

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

// Command functions
func performDevScan(scanner *Scanner) tea.Cmd {
	return func() tea.Msg {
		scanners := []struct {
			name string
			fn   func() *ScanResult
		}{
			{"Node Modules", scanner.scanNodeModules},
			{"NPM/Yarn/PNPM Caches", scanner.scanNpmYarnCaches},
			{"Python Artifacts", scanner.scanPythonArtifacts},
			{"Go Artifacts", scanner.scanGoArtifacts},
			{"Rust Artifacts", scanner.scanRustArtifacts},
			{"Java/JVM Artifacts", scanner.scanJavaArtifacts},
			{"Ruby Artifacts", scanner.scanRubyArtifacts},
			{"Docker Artifacts", scanner.scanDockerArtifacts},
			{"IDE Caches", scanner.scanIDECaches},
			{"Xcode Files", scanner.scanXcodeFiles},
			{"Homebrew Cache", scanner.scanBrewCache},
			{"CocoaPods", scanner.scanCocoaPods},
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

		return cleanCompleteMsg{freed: freed}
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

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
