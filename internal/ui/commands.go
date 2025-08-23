package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rahulvramesh/cleanWithCli/internal/scanner"
	"github.com/rahulvramesh/cleanWithCli/internal/types"
)

// Channel for sending scan updates
var scanUpdateChan chan types.ScanProgressMsg

// Command functions
func performDevScan(s *scanner.Scanner) tea.Cmd {
	return func() tea.Msg {
		// Create a channel for live updates
		scanUpdateChan = make(chan types.ScanProgressMsg, 100)

		// Start a goroutine to send updates
		go func() {
			for range scanUpdateChan {
				// Updates are being handled by the channel
			}
		}()

		// Note: Deep scan operations run in parallel but may take longer
		// due to traversing entire home directory
		scanners := []struct {
			name string
			fn   func() *types.ScanResult
		}{
			{"Node Modules", s.ScanNodeModules},
			{"Python Artifacts", s.ScanPythonArtifacts},
			{"Rust Artifacts", s.ScanRustArtifacts},
			{"Build Artifacts", s.ScanBuildArtifacts},
			{"NPM/Yarn/PNPM Caches", s.ScanNpmYarnCaches},
			{"Go Artifacts", s.ScanGoArtifacts},
			{"Java/JVM Artifacts", s.ScanJavaArtifacts},
			{"Ruby Artifacts", s.ScanRubyArtifacts},
			{"Docker Artifacts", s.ScanDockerArtifacts},
			{"IDE Caches", s.ScanIDECaches},
			{"Xcode Files", s.ScanXcodeFiles},
			{"Homebrew Cache", s.ScanBrewCache},
			{"CocoaPods", s.ScanCocoaPods},
		}

		results := make(map[string]*types.ScanResult)
		var totalSize int64
		var totalFound int

		// Use goroutines for parallel scanning
		var wg sync.WaitGroup
		for _, sc := range scanners {
			wg.Add(1)
			go func(name string, scanFunc func() *types.ScanResult) {
				defer wg.Done()

				result := scanFunc()
				if result.Total > 0 {
					results[name] = result
					totalSize += result.Total
					totalFound += len(result.Items)
				}
			}(sc.name, sc.fn)
		}

		wg.Wait()
		close(scanUpdateChan)

		return types.ScanCompleteMsg{
			Results:   results,
			TotalSize: totalSize,
		}
	}
}

func performScan(s *scanner.Scanner) tea.Cmd {
	return func() tea.Msg {
		scanners := []struct {
			name string
			fn   func() *types.ScanResult
		}{
			{"Cache Files", s.ScanCacheFiles},
			{"Log Files", s.ScanLogFiles},
			{"Trash", s.ScanTrash},
			{"Old Downloads", s.ScanDownloads},
			{"Xcode Files", s.ScanXcodeFiles},
			{"Homebrew Cache", s.ScanBrewCache},
			{"Node Modules", s.ScanNodeModules},
		}

		results := make(map[string]*types.ScanResult)
		var totalSize int64
		var completed int32

		// Use goroutines for parallel scanning
		var wg sync.WaitGroup
		for _, sc := range scanners {
			wg.Add(1)
			go func(name string, scanFunc func() *types.ScanResult) {
				defer wg.Done()

				result := scanFunc()
				if result.Total > 0 {
					results[name] = result
					totalSize += result.Total
				}

				atomic.AddInt32(&completed, 1)
			}(sc.name, sc.fn)
		}

		wg.Wait()

		return types.ScanCompleteMsg{
			Results:   results,
			TotalSize: totalSize,
		}
	}
}

func showDiskUsage() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("df", "-h")
		output, err := cmd.Output()
		if err != nil {
			return types.ErrMsg{Err: err}
		}

		// Parse the disk usage output and create a table
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) == 0 {
			return types.ErrMsg{Err: fmt.Errorf("no disk usage data")}
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

		return types.DiskUsageMsg{Table: t}
	}
}

func exploreDirectory(m *Model, dirPath string) tea.Cmd {
	return func() tea.Msg {
		// This would need to be implemented to explore directories
		// For now, just return an error
		return types.ErrMsg{Err: fmt.Errorf("directory exploration not implemented")}
	}
}

func cleanProgressTicker() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return nil // This will trigger the cleaning progress update
	})
}

func performCleanMarkedItemsWithProgress(s *scanner.Scanner, markedItems map[string]bool, detailItems []types.FileItem) tea.Cmd {
	return func() tea.Msg {
		var freed int64
		var paths []string

		for path := range markedItems {
			err := os.RemoveAll(path)
			if err == nil {
				// Find the size of the deleted item
				for _, item := range detailItems {
					if item.Path == path {
						freed += item.Size
						paths = append(paths, path)
						break
					}
				}
			}
		}

		cleaningInProgress = false
		return types.BatchCleanCompleteMsg{
			Freed: freed,
			Paths: paths,
		}
	}
}

func performCleanItemWithProgress(s *scanner.Scanner, item types.FileItem) tea.Cmd {
	return func() tea.Msg {
		err := os.RemoveAll(item.Path)
		var freed int64
		if err == nil {
			freed = item.Size
		}

		cleaningInProgress = false
		return types.CleanCompleteMsg{
			Freed: freed,
			Path:  item.Path,
		}
	}
}
