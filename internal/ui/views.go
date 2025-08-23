package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"

	"github.com/rahulvramesh/cleanWithCli/internal/utils"
)

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// View renders the UI
func (m Model) View() string {
	var s strings.Builder

	// Header with padding
	header := TitleStyle.Render("🧹 Mac Storage Cleaner")
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
		errMsg := lipgloss.NewStyle().Padding(0, 3).Render(ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
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

	s.WriteString(HeaderStyle.Render("Main Menu"))
	s.WriteString("\n\n\n")

	for i, item := range items {
		cursor := "  "
		style := lipgloss.NewStyle()

		if m.menuChoice == i {
			cursor = "▸ "
			style = SelectedStyle
		}

		s.WriteString("  " + cursor + style.Render(item) + "\n\n")
	}

	s.WriteString("\n\n")
	s.WriteString(DimStyle.Render("Use ↑/↓ or j/k to navigate, Enter to select, q to quit"))

	return s.String()
}

func (m Model) renderScanning() string {
	var s strings.Builder

	s.WriteString(HeaderStyle.Render("Scanning System..."))
	s.WriteString("\n\n")

	// Show scanning stats
	if m.scanFoundItems > 0 {
		stats := fmt.Sprintf("🔍 Found %d items | %s total",
			m.scanFoundItems,
			humanize.Bytes(uint64(m.scanTotalSize)))
		s.WriteString("  " + SuccessStyle.Render(stats))
		s.WriteString("\n\n")
	}

	s.WriteString("  " + m.spinner.View() + " " + m.scanMessage)
	s.WriteString("\n\n")

	// Show recently scanned paths
	if len(m.scanningPaths) > 0 {
		s.WriteString("  " + DimStyle.Render("📁 Recently found:"))
		s.WriteString("\n")
		for _, path := range m.scanningPaths {
			// Truncate long paths
			displayPath := path
			if len(path) > 60 {
				displayPath = "..." + path[len(path)-57:]
			}
			s.WriteString("     " + DimStyle.Render(displayPath))
			s.WriteString("\n")
		}
		s.WriteString("\n")
	}

	// Show what we're looking for
	if m.state == "scanning" && strings.Contains(m.scanMessage, "Dev") {
		s.WriteString("  " + DimStyle.Render("🎯 Searching for:"))
		s.WriteString("\n")
		s.WriteString("  " + DimStyle.Render("   • node_modules, venv, __pycache__"))
		s.WriteString("\n")
		s.WriteString("  " + DimStyle.Render("   • build/dist folders, target directories"))
		s.WriteString("\n")
		s.WriteString("  " + DimStyle.Render("   • Package manager caches"))
		s.WriteString("\n\n")
	}

	s.WriteString(DimStyle.Render("Please wait, scanning your directories..."))

	return s.String()
}

func (m Model) renderResults() string {
	var s strings.Builder

	s.WriteString(HeaderStyle.Render("Scan Results"))
	s.WriteString("\n\n\n")

	if len(m.results) == 0 {
		s.WriteString("  " + WarningStyle.Render("No cleanable files found"))
		return s.String()
	}

	// Create table
	categories := utils.GetSortedCategories(m.results)

	s.WriteString("  Category                    Items        Size\n")
	s.WriteString("  ─────────────────────────────────────────────\n")

	for i, category := range categories {
		result := m.results[category]
		cursor := "  "
		style := lipgloss.NewStyle()

		if m.menuChoice == i {
			cursor = "▸ "
			style = SelectedStyle
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
	s.WriteString("    " + SuccessStyle.Render(totalLine) + "\n\n")

	// Back option
	cursor := "  "
	style := lipgloss.NewStyle()
	if m.menuChoice == len(categories) {
		cursor = "▸ "
		style = SelectedStyle
	}
	s.WriteString("  " + cursor + style.Render("← Back to Menu") + "\n")

	s.WriteString("\n\n")
	s.WriteString(DimStyle.Render("Press Enter to explore category • ESC to go back to menu"))

	return s.String()
}

func (m Model) renderCleaning() string {
	var s strings.Builder

	s.WriteString(HeaderStyle.Render("Cleaning Files..."))
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

	s.WriteString(HeaderStyle.Render("Disk Usage Report"))
	s.WriteString("\n\n\n")
	s.WriteString(m.diskUsageTable.View())
	s.WriteString("\n\n\n")
	s.WriteString(DimStyle.Render("Use ↑/↓ or j/k to navigate, ESC or q to go back to menu"))

	return s.String()
}

func (m Model) renderDetail() string {
	var s strings.Builder

	// Breadcrumb navigation
	breadcrumb := strings.Join(m.currentPath, " > ")
	s.WriteString(HeaderStyle.Render("📁 " + breadcrumb))
	s.WriteString("\n\n")

	// Show success message if item was just cleaned
	if m.state == "detail" && strings.Contains(m.scanMessage, "✅") {
		s.WriteString("  " + SuccessStyle.Render(m.scanMessage))
		s.WriteString("\n")
	}
	s.WriteString("\n")

	if len(m.detailItems) == 0 {
		s.WriteString("  " + DimStyle.Render("No items found"))
		s.WriteString("\n\n")
		s.WriteString(DimStyle.Render("Press Backspace to go back"))
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
		s.WriteString("  " + DimStyle.Render(scrollInfo))
		if startIdx > 0 {
			s.WriteString(DimStyle.Render(" ↑"))
		}
		if endIdx < len(m.detailItems) {
			s.WriteString(DimStyle.Render(" ↓"))
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
			style = SelectedStyle
		}

		// Checkbox indicator
		checkbox := "☐"
		if m.markedItems[item.Path] {
			checkbox = "☑️"
		}

		icon := "📄"
		if item.IsDir {
			icon = "📁"
		}

		// Adjust name width based on terminal width (accounting for checkbox)
		nameWidth := min(45, m.width-35)
		line := fmt.Sprintf("%s %s %-*s %10s",
			checkbox,
			icon,
			nameWidth,
			utils.TruncatePath(item.Name, nameWidth),
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
	s.WriteString("  " + DimStyle.Render(fmt.Sprintf("Total: %s", humanize.Bytes(uint64(totalSize)))))

	// Show marked items status
	markedCount := len(m.markedItems)
	if markedCount > 0 {
		var markedSize int64
		for path := range m.markedItems {
			// Find the size of marked items
			for _, item := range m.detailItems {
				if item.Path == path {
					markedSize += item.Size
					break
				}
			}
		}
		s.WriteString(" • ")
		s.WriteString(SuccessStyle.Render(fmt.Sprintf("Marked: %d items (%s)", markedCount, humanize.Bytes(uint64(markedSize)))))
	}
	s.WriteString("\n\n")

	// Instructions
	s.WriteString(DimStyle.Render("↑/↓ Navigate • Space: Mark • Shift+A: Mark All • Shift+N: Unmark All • Shift+D: Delete Marked • c: Clean • ESC: Back"))

	return s.String()
}

func (m Model) getTotalItems() int {
	total := 0
	for _, result := range m.results {
		total += len(result.Items)
	}
	return total
}
