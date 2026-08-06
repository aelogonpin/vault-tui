package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorAccent = lipgloss.Color("39")  // blue
	colorGood   = lipgloss.Color("42")  // green
	colorWarn   = lipgloss.Color("214") // orange
	colorBad    = lipgloss.Color("203") // red
	colorMuted  = lipgloss.Color("245") // grey
	colorJSON   = lipgloss.Color("135") // purple

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("230")).
			Background(colorAccent).
			Padding(0, 1)

	breadcrumbStyle = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)

	helpStyle = lipgloss.NewStyle().Foreground(colorMuted)

	statusOKStyle   = lipgloss.NewStyle().Foreground(colorGood)
	statusErrStyle  = lipgloss.NewStyle().Foreground(colorBad).Bold(true)
	statusWarnStyle = lipgloss.NewStyle().Foreground(colorWarn)

	labelStyle = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(0, 1)

	dangerBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBad).
			Padding(0, 1)

	focusedFieldStyle = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	blurredFieldStyle = lipgloss.NewStyle().Foreground(colorMuted)

	jsonTagStyle      = lipgloss.NewStyle().Foreground(colorJSON).Italic(true)
	columnHeaderStyle = lipgloss.NewStyle().Foreground(colorMuted).Bold(true).Underline(true)
)
