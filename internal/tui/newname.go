package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) updateNewName(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.newNameInput.Blur()
		m.screen = m.prevScreen
		return m, nil
	case "enter":
		name := strings.Trim(strings.TrimSpace(m.newNameInput.Value()), "/")
		if name == "" {
			if m.newNameIsFolder {
				m.status = statusWarnStyle.Render("Enter a name for the new folder.")
			} else {
				m.status = statusWarnStyle.Render("Enter a name for the new secret.")
			}
			return m, nil
		}
		full := joinPath(m.currentPath, name)
		m.newNameInput.Blur()
		m.status = ""
		m.err = nil

		if m.newNameIsFolder {
			m.screen = screenBrowse
			return m, createFolderCmd(m.client, m.currentMount, full)
		}

		m.editor = newEditorModel(full, nil)
		m.prevScreen = screenBrowse
		m.screen = screenEditor
		return m, nil
	}

	var cmd tea.Cmd
	m.newNameInput, cmd = m.newNameInput.Update(msg)
	return m, cmd
}

func (m Model) viewNewName() string {
	title := titleStyle.Render(" vault-tui ") + "  " + m.breadcrumb()
	label := "New secret path (relative to current folder, may include /):"
	if m.newNameIsFolder {
		label = "New folder name (relative to current folder, may include / for nesting):"
	}
	prompt := labelStyle.Render(label) + "\n" + m.newNameInput.View()
	help := helpStyle.Render("enter confirm • esc cancel")
	return lipgloss.JoinVertical(lipgloss.Left, title, boxStyle.Render(prompt), help)
}
