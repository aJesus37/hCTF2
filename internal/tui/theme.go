package tui

import "github.com/charmbracelet/lipgloss"

var (
	HeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	CellStyle    = lipgloss.NewStyle().PaddingRight(2)
	SolvedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	ErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	MutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
)
