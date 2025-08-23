package ui

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustin/go-humanize"

	"github.com/rahulvramesh/cleanWithCli/internal/types"
	"github.com/rahulvramesh/cleanWithCli/internal/utils"
)

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// cleaningInProgress tracks if cleaning is currently in progress
var cleaningInProgress bool

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
					categories := utils.GetSortedCategories(m.results)
					if m.menuChoice < len(categories) {
						category := categories[m.menuChoice]
						m.currentCategory = category
						m.currentPath = []string{category}
						m.detailItems = m.results[category].Items
						m.detailChoice = 0
						m.detailOffset = 0
						m.markedItems = make(map[string]bool) // Reset marked items
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
				m.markedItems = make(map[string]bool) // Reset marked items
			} else if m.state == "results" || m.state == "cleaning" || m.state == "diskusage" {
				m.state = "menu"
				m.menuChoice = 0
			}

		case " ": // Space key
			// Toggle marking of selected item in detail view
			if m.state == "detail" && m.detailChoice < len(m.detailItems) {
				item := m.detailItems[m.detailChoice]
				if m.markedItems[item.Path] {
					delete(m.markedItems, item.Path)
				} else {
					m.markedItems[item.Path] = true
				}
			}

		case "A": // Shift+A
			// Mark all items in detail view
			if m.state == "detail" {
				for _, item := range m.detailItems {
					m.markedItems[item.Path] = true
				}
			}

		case "N": // Shift+N
			// Unmark all items
			if m.state == "detail" {
				m.markedItems = make(map[string]bool)
			}

		case "D": // Shift+D
			// Delete marked items
			if m.state == "detail" && len(m.markedItems) > 0 {
				m.state = "cleaning"
				m.cleanProgress = 0.0
				m.scanMessage = fmt.Sprintf("Starting to clean %d marked items...", len(m.markedItems))
				return m, tea.Batch(
					m.spinner.Tick,
					cleanProgressTicker(),
					performCleanMarkedItemsWithProgress(m.scanner, m.markedItems, m.detailItems),
				)
			}

		case "c":
			// Clean selected item in detail view
			if m.state == "detail" && m.detailChoice < len(m.detailItems) {
				item := m.detailItems[m.detailChoice]
				m.state = "cleaning"
				m.cleanProgress = 0.0
				m.scanMessage = fmt.Sprintf("Cleaning %s...", item.Name)
				return m, tea.Batch(
					m.spinner.Tick,
					cleanProgressTicker(),
					performCleanItemWithProgress(m.scanner, item),
				)
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case types.ScanProgressMsg:
		m.scanProgress = msg.Percent
		m.scanMessage = msg.Message
		if msg.Path != "" {
			// Add to scanning paths (keep last 10)
			m.scanningPaths = append(m.scanningPaths, msg.Path)
			if len(m.scanningPaths) > 10 {
				m.scanningPaths = m.scanningPaths[len(m.scanningPaths)-10:]
			}
			m.scanFoundItems = msg.Found
			m.scanTotalSize += msg.Size
		}
		return m, nil

	case types.CleanProgressMsg:
		m.cleanProgress = msg.Percent / 100.0
		m.scanMessage = msg.Message
		// Continue the ticker if cleaning is still in progress
		if cleaningInProgress {
			return m, cleanProgressTicker()
		}
		return m, nil

	case types.ScanCompleteMsg:
		m.results = msg.Results
		m.totalSize = msg.TotalSize
		m.state = "results"
		m.menuChoice = 0
		return m, nil

	case types.CleanCompleteMsg:
		if m.state == "cleaning" {
			// If we were in detail view, refresh it
			if msg.Path != "" {
				// Remove the deleted item from the list
				newItems := []types.FileItem{}
				for _, item := range m.detailItems {
					if item.Path != msg.Path {
						newItems = append(newItems, item)
					}
				}
				m.detailItems = newItems

				// Remove from marked items if it was marked
				delete(m.markedItems, msg.Path)

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
						newCategoryItems := []types.FileItem{}
						for _, item := range result.Items {
							if item.Path != msg.Path {
								newCategoryItems = append(newCategoryItems, item)
							}
						}
						result.Items = newCategoryItems
						result.Total -= msg.Freed
					}
				}

				m.totalSize -= msg.Freed
				m.state = "detail" // Return to detail view

				// Show success message briefly
				deletedName := filepath.Base(msg.Path)
				m.scanMessage = fmt.Sprintf("✅ Deleted %s (%s)", deletedName, humanize.Bytes(uint64(msg.Freed)))
			} else {
				// Regular cleaning from results view
				m.totalSize -= msg.Freed
				m.state = "results"
			}
		}
		return m, nil

	case types.BatchCleanCompleteMsg:
		if m.state == "cleaning" {
			// Remove all deleted items from the list
			newItems := []types.FileItem{}
			for _, item := range m.detailItems {
				isDeleted := false
				for _, deletedPath := range msg.Paths {
					if item.Path == deletedPath {
						isDeleted = true
						break
					}
				}
				if !isDeleted {
					newItems = append(newItems, item)
				}
			}
			m.detailItems = newItems

			// Clear marked items for deleted paths
			for _, deletedPath := range msg.Paths {
				delete(m.markedItems, deletedPath)
			}

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

			// Update the total size in results
			if m.currentCategory != "" {
				if result, exists := m.results[m.currentCategory]; exists {
					// Update the category's items list
					newCategoryItems := []types.FileItem{}
					for _, item := range result.Items {
						isDeleted := false
						for _, deletedPath := range msg.Paths {
							if item.Path == deletedPath {
								isDeleted = true
								break
							}
						}
						if !isDeleted {
							newCategoryItems = append(newCategoryItems, item)
						}
					}
					result.Items = newCategoryItems
					result.Total -= msg.Freed
				}
			}

			m.totalSize -= msg.Freed
			m.state = "detail" // Return to detail view

			// Show success message
			m.scanMessage = fmt.Sprintf("✅ Deleted %d items (%s)", len(msg.Paths), humanize.Bytes(uint64(msg.Freed)))
		}
		return m, nil

	case types.DiskUsageMsg:
		m.diskUsageTable = msg.Table
		m.state = "diskusage"
		return m, nil

	case types.ErrMsg:
		m.err = msg
		return m, nil

	default:
		// Handle nil messages from tickers
		if msg == nil && cleaningInProgress {
			return m, cleanProgressTicker()
		}
	}

	return m, nil
}
