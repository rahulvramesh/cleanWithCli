package ui

import "github.com/charmbracelet/lipgloss"

// Styles
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			Background(lipgloss.Color("235")).
			Padding(0, 1)

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229"))

	SelectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57"))

	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	SuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("46"))

	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226"))
)
