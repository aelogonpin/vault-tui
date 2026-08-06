package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) selectedEntry() (entryItem, bool) {
	item, ok := m.entryList.SelectedItem().(entryItem)
	return item, ok
}

// destinationInsideSource reports whether dstPath is the same path as the
// armed picker's source, or — for a folder op — a subfolder underneath it.
// Pasting a folder into its own subtree would have the delete-the-copy
// self-destruction bug that TestArmMoveThenPasteAtSourceIsRefused guards
// against for single secrets, just one level removed: MoveFolder deletes
// every source secret only after all copies succeed, so a copy landing
// inside the very subtree about to be deleted would vanish along with it.
func destinationInsideSource(picker *pendingOp, dstPath string) bool {
	src := strings.Trim(picker.srcPath, "/")
	dst := strings.Trim(dstPath, "/")
	if dst == src {
		return true
	}
	return picker.isFolder && strings.HasPrefix(dst, src+"/")
}

func (m Model) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// While the filter input has focus, every keystroke belongs to it —
	// don't let list shortcuts below (q, n, e, r, d, D, h...) hijack it.
	if m.entryList.SettingFilter() {
		var cmd tea.Cmd
		m.entryList, cmd = m.entryList.Update(msg)
		return m, cmd
	}

	// A copy/move is armed: 'esc' cancels the whole operation, while plain
	// navigation (backspace/h to go up, entering folders/mounts) keeps its
	// usual meaning — you're still moving around to find the destination.
	// 'p' pastes into whatever folder we're currently looking at.
	if m.picker != nil {
		switch msg.String() {
		case "esc":
			m.picker = nil
			m.status = statusWarnStyle.Render("Cancelled.")
			return m, nil
		case "p":
			dstPath := joinPath(m.currentPath, lastSegment(m.picker.srcPath))
			if m.currentMount == m.picker.srcMount && destinationInsideSource(m.picker, dstPath) {
				msg := "That's the source — navigate to a different destination first."
				if m.picker.isFolder {
					msg = "Can't paste a folder into itself or one of its own subfolders — navigate to a different destination first."
				}
				m.status = statusWarnStyle.Render(msg)
				return m, nil
			}
			op := *m.picker
			m.err = nil
			return m, pasteCheckCmd(m.client, op, m.currentMount, dstPath)
		}
	}

	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit

	case "enter":
		if entry, ok := m.selectedEntry(); ok {
			full := joinPath(m.currentPath, entry.entry.Name)
			if entry.entry.IsFolder {
				m.currentPath = full
				m.err = nil
				m.loadingEntries = true
				return m, loadEntriesCmd(m.client, m.currentMount, m.currentPath)
			}
			m.secretPath = full
			m.err = nil
			m.status = ""
			m.showVersions = false
			m.viewingVersion = 0
			m.screen = screenSecret
			m.loadingSecret = true
			return m, loadSecretCmd(m.client, m.currentMount, full, 0)
		}

	case "backspace", "esc", "h":
		if m.currentPath == "" {
			m.screen = screenMounts
			m.err = nil
			return m, nil
		}
		m.currentPath = parentPath(m.currentPath)
		m.loadingEntries = true
		return m, loadEntriesCmd(m.client, m.currentMount, m.currentPath)

	case "n":
		m.newNameInput.SetValue("")
		m.newNameInput.Focus()
		m.newNameIsFolder = false
		m.prevScreen = screenBrowse
		m.screen = screenNewName
		m.err = nil
		return m, nil

	case "N":
		m.newNameInput.SetValue("")
		m.newNameInput.Focus()
		m.newNameIsFolder = true
		m.prevScreen = screenBrowse
		m.screen = screenNewName
		m.err = nil
		return m, nil

	case "e":
		if entry, ok := m.selectedEntry(); ok && !entry.entry.IsFolder {
			full := joinPath(m.currentPath, entry.entry.Name)
			m.secretPath = full
			m.pendingEditAfterLoad = true
			m.screen = screenSecret
			m.loadingSecret = true
			m.err = nil
			return m, loadSecretCmd(m.client, m.currentMount, full, 0)
		}

	case "r":
		if entry, ok := m.selectedEntry(); ok {
			m.renameOldPath = joinPath(m.currentPath, entry.entry.Name)
			m.renameParent = m.currentPath
			m.renameIsFolder = entry.entry.IsFolder
			m.renameInput.SetValue(entry.entry.Name)
			m.renameInput.CursorEnd()
			m.renameInput.Focus()
			m.prevScreen = screenBrowse
			m.screen = screenRename
			m.err = nil
			return m, nil
		}

	case "c":
		if entry, ok := m.selectedEntry(); ok {
			full := joinPath(m.currentPath, entry.entry.Name)
			m.picker = &pendingOp{kind: "copy", srcMount: m.currentMount, srcPath: full, isFolder: entry.entry.IsFolder}
			m.err = nil
			what := "\"" + full + "\""
			if entry.entry.IsFolder {
				what = "folder \"" + full + "\" (and everything inside it)"
			}
			m.status = statusWarnStyle.Render("Copying " + what + " — browse to a destination folder (any mount you can reach), then p to paste, esc to cancel.")
			return m, nil
		}

	case "x":
		if entry, ok := m.selectedEntry(); ok {
			full := joinPath(m.currentPath, entry.entry.Name)
			m.picker = &pendingOp{kind: "move", srcMount: m.currentMount, srcPath: full, isFolder: entry.entry.IsFolder}
			m.err = nil
			what := "\"" + full + "\""
			if entry.entry.IsFolder {
				what = "folder \"" + full + "\" (and everything inside it)"
			}
			m.status = statusWarnStyle.Render("Moving " + what + " — browse to a destination folder (any mount you can reach), then p to paste, esc to cancel.")
			return m, nil
		}

	case "d":
		if entry, ok := m.selectedEntry(); ok && !entry.entry.IsFolder {
			full := joinPath(m.currentPath, entry.entry.Name)
			m.confirmMessage = "Soft-delete latest version of \"" + full + "\"?\n(Recoverable — undelete via the Vault CLI/API until it's destroyed.)"
			m.confirmDanger = false
			m.confirmAction = deleteSecretCmd(m.client, m.currentMount, full)
			m.confirmBack = screenBrowse
			m.prevScreen = screenBrowse
			m.screen = screenConfirm
			m.err = nil
			return m, nil
		}

	case "D":
		if entry, ok := m.selectedEntry(); ok && !entry.entry.IsFolder {
			full := joinPath(m.currentPath, entry.entry.Name)
			m.confirmMessage = "PERMANENTLY destroy \"" + full + "\" and ALL of its version history?\nThis cannot be undone."
			m.confirmDanger = true
			m.confirmAction = destroySecretCmd(m.client, m.currentMount, full)
			m.confirmBack = screenBrowse
			m.prevScreen = screenBrowse
			m.screen = screenConfirm
			m.err = nil
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.entryList, cmd = m.entryList.Update(msg)
	return m, cmd
}

func (m Model) viewBrowse() string {
	title := titleStyle.Render(" vault-tui ") + "  " + m.breadcrumb()
	body := m.entryList.View()
	if m.loadingEntries {
		body = "Loading..."
	} else if len(m.entries) == 0 {
		body = helpStyle.Render("(empty — press n for a new secret, N for a new folder)")
	}
	return lipgloss.JoinVertical(lipgloss.Left, title, body, m.browseHelp())
}

// browseHelp builds the shortcut line for whatever is actually valid on
// the currently selected entry: folders can't be edited/deleted directly
// here (only renamed, or opened and dealt with secret by secret), so those
// shortcuts only show up once a secret is selected.
func (m Model) browseHelp() string {
	if m.picker != nil {
		return helpStyle.Render("enter/backspace/h navigate • p paste here • esc cancel " + m.picker.kind)
	}

	parts := []string{"enter open", "n new secret", "N new folder"}
	if entry, ok := m.selectedEntry(); ok {
		if !entry.entry.IsFolder {
			parts = append(parts, "e edit")
		}
		parts = append(parts, "c copy", "x move", "r rename")
		if !entry.entry.IsFolder {
			parts = append(parts, "d delete", "D destroy")
		}
	}
	parts = append(parts, "esc back", "q quit")
	return helpStyle.Render(strings.Join(parts, " • "))
}
