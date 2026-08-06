package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) updateMounts(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// While the filter input has focus, every keystroke belongs to it —
	// don't let list shortcuts below (q, etc.) hijack it.
	if m.mountList.SettingFilter() {
		var cmd tea.Cmd
		m.mountList, cmd = m.mountList.Update(msg)
		return m, cmd
	}

	if m.picker != nil && msg.String() == "esc" {
		m.picker = nil
		m.status = statusWarnStyle.Render("Cancelled.")
		return m, nil
	}

	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "enter":
		if item, ok := m.mountList.SelectedItem().(mountItem); ok {
			if item.mount.Version != "2" {
				m.status = statusWarnStyle.Render("Only KV v2 mounts are supported for browsing/editing.")
				return m, nil
			}
			m.currentMount = item.mount.Path
			m.currentPath = ""
			m.err = nil
			if m.picker == nil {
				m.status = ""
			}
			m.screen = screenBrowse
			m.loadingEntries = true
			return m, loadEntriesCmd(m.client, m.currentMount, m.currentPath)
		}
	}

	var cmd tea.Cmd
	m.mountList, cmd = m.mountList.Update(msg)
	return m, cmd
}

func (m Model) viewMounts() string {
	title := titleStyle.Render(" vault-tui ") + "  " + m.breadcrumb()
	help := helpStyle.Render("↑/↓ navigate • enter select • / filter • q quit")
	if m.picker != nil {
		help = helpStyle.Render("enter select destination mount • esc cancel " + m.picker.kind)
	}
	return lipgloss.JoinVertical(lipgloss.Left, title, m.mountList.View(), help)
}
