package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) updateRename(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.renameInput.Blur()
		m.screen = m.prevScreen
		m.err = nil
		return m, nil

	case "enter":
		newLeaf := strings.Trim(strings.TrimSpace(m.renameInput.Value()), "/")
		if newLeaf == "" {
			m.status = statusWarnStyle.Render("Enter a name.")
			return m, nil
		}
		newPath := joinPath(m.renameParent, newLeaf)
		if newPath == strings.Trim(m.renameOldPath, "/") {
			m.status = statusWarnStyle.Render("Enter a different name.")
			return m, nil
		}
		m.renameInput.Blur()
		m.err = nil
		if m.renameIsFolder {
			m.status = statusWarnStyle.Render("Renaming folder — copying every secret underneath, this may take a moment...")
			m.screen = screenBrowse
			return m, renameFolderCmd(m.client, m.currentMount, m.renameOldPath, newPath)
		}
		m.status = ""
		m.screen = screenBrowse
		return m, renameSecretCmd(m.client, m.currentMount, m.renameOldPath, newPath)
	}

	var cmd tea.Cmd
	m.renameInput, cmd = m.renameInput.Update(msg)
	return m, cmd
}

func (m Model) viewRename() string {
	title := titleStyle.Render(" vault-tui ") + "  " + m.breadcrumb()
	kind := "secret"
	if m.renameIsFolder {
		kind = "folder"
	}

	parentDisplay := m.renameParent + "/"
	if m.renameParent == "" {
		parentDisplay = "(mount root)/"
	}

	warning := ""
	if !m.renameIsFolder {
		warning = "\n" + statusWarnStyle.Render("Note: version history is not preserved across a rename (copy + delete).")
	} else {
		warning = "\n" + statusWarnStyle.Render("This moves every secret under this folder to the new prefix (copy + delete),\nand does not preserve their version history.")
	}

	label := labelStyle.Render("Rename " + kind + " \"" + lastSegment(m.renameOldPath) + "\" — parent path is locked:")
	inputLine := blurredFieldStyle.Render(parentDisplay) + m.renameInput.View()
	prompt := label + "\n" + inputLine + warning
	help := helpStyle.Render("enter confirm • esc cancel")
	return lipgloss.JoinVertical(lipgloss.Left, title, boxStyle.Render(prompt), help)
}
