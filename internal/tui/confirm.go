package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		m.status = statusWarnStyle.Render("Working...")
		m.err = nil
		action := m.confirmAction
		m.screen = m.confirmBack
		return m, action
	case "n", "N", "esc":
		m.screen = m.prevScreen
		return m, nil
	}
	return m, nil
}

func (m Model) viewConfirm() string {
	title := titleStyle.Render(" vault-tui ") + "  " + m.breadcrumb()
	style := boxStyle
	if m.confirmDanger {
		style = dangerBoxStyle
	}
	help := helpStyle.Render("y confirm • n/esc cancel")
	return lipgloss.JoinVertical(lipgloss.Left, title, style.Render(m.confirmMessage), help)
}
