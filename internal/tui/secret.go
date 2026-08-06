package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// maskedValueMarker is shown in place of every secret value until the user
// explicitly reveals them with 'm'. A fixed-width marker (rather than one
// sized to the real value) avoids leaking length as a side channel.
const maskedValueMarker = "••••••••"

func (m Model) updateSecret(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showVersions {
		if m.versionList.SettingFilter() {
			var cmd tea.Cmd
			m.versionList, cmd = m.versionList.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "esc", "v":
			m.showVersions = false
			return m, nil
		case "enter":
			if item, ok := m.versionList.SelectedItem().(versionItem); ok {
				m.viewingVersion = item.v.Version
				m.loadingSecret = true
				m.showVersions = false
				return m, loadSecretCmd(m.client, m.currentMount, m.secretPath, item.v.Version)
			}
		}
		var cmd tea.Cmd
		m.versionList, cmd = m.versionList.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit

	case "esc", "backspace", "h":
		m.screen = screenBrowse
		m.err = nil
		return m, nil

	case "v":
		m.showVersions = true
		return m, nil

	case "m":
		m.revealSecret = !m.revealSecret
		return m, nil

	case "e":
		var data map[string]interface{}
		if m.secret != nil {
			data = m.secret.Data
		}
		m.editor = newEditorModel(m.secretPath, data)
		m.prevScreen = screenSecret
		m.screen = screenEditor
		m.err = nil
		return m, nil

	case "r":
		m.renameOldPath = m.secretPath
		m.renameIsFolder = false
		m.renameParent = parentPath(m.secretPath)
		m.renameInput.SetValue(lastSegment(m.secretPath))
		m.renameInput.CursorEnd()
		m.renameInput.Focus()
		m.prevScreen = screenSecret
		m.screen = screenRename
		m.err = nil
		return m, nil

	case "c":
		m.picker = &pendingOp{kind: "copy", srcMount: m.currentMount, srcPath: m.secretPath}
		m.screen = screenBrowse
		m.err = nil
		m.status = statusWarnStyle.Render("Copying \"" + m.secretPath + "\" — browse to a destination folder (any mount you can reach), then p to paste, esc to cancel.")
		return m, nil

	case "x":
		m.picker = &pendingOp{kind: "move", srcMount: m.currentMount, srcPath: m.secretPath}
		m.screen = screenBrowse
		m.err = nil
		m.status = statusWarnStyle.Render("Moving \"" + m.secretPath + "\" — browse to a destination folder (any mount you can reach), then p to paste, esc to cancel.")
		return m, nil

	case "d":
		m.confirmMessage = "Soft-delete latest version of \"" + m.secretPath + "\"?\n(Recoverable — undelete via the Vault CLI/API until it's destroyed.)"
		m.confirmDanger = false
		m.confirmAction = deleteSecretCmd(m.client, m.currentMount, m.secretPath)
		m.confirmBack = screenSecret
		m.prevScreen = screenSecret
		m.screen = screenConfirm
		m.err = nil
		return m, nil

	case "D":
		m.confirmMessage = "PERMANENTLY destroy \"" + m.secretPath + "\" and ALL of its version history?\nThis cannot be undone."
		m.confirmDanger = true
		m.confirmAction = destroySecretCmd(m.client, m.currentMount, m.secretPath)
		m.confirmBack = screenBrowse
		m.prevScreen = screenSecret
		m.screen = screenConfirm
		m.err = nil
		return m, nil
	}

	return m, nil
}

func (m Model) viewSecret() string {
	title := titleStyle.Render(" vault-tui ") + "  " + m.breadcrumb() + breadcrumbStyle.Render(" › "+lastSegment(m.secretPath))

	if m.showVersions {
		help := helpStyle.Render("↑/↓ navigate • enter view version • esc/v back")
		return lipgloss.JoinVertical(lipgloss.Left, title, m.versionList.View(), help)
	}

	var b strings.Builder
	if m.loadingSecret {
		b.WriteString("Loading...\n")
	} else if m.secret == nil {
		b.WriteString(helpStyle.Render("(no data)") + "\n")
	} else {
		versionLabel := "latest"
		if m.viewingVersion != 0 {
			versionLabel = fmt.Sprintf("v%d", m.viewingVersion)
		} else if m.secret.VersionMetadata != nil {
			versionLabel = fmt.Sprintf("v%d (latest)", m.secret.VersionMetadata.Version)
		}
		b.WriteString(labelStyle.Render("Version: ") + versionLabel + "\n")

		if len(m.secret.Data) == 0 {
			b.WriteString(helpStyle.Render("(no key/value pairs)") + "\n")
		} else {
			keys := make([]string, 0, len(m.secret.Data))
			for k := range m.secret.Data {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				value := maskedValueMarker
				if m.revealSecret {
					value = toDisplayString(m.secret.Data[k])
				}
				b.WriteString(labelStyle.Render(k+": ") + value + "\n")
			}
			if m.revealSecret {
				b.WriteString("\n" + statusWarnStyle.Render("m to hide values"))
			} else {
				b.WriteString("\n" + helpStyle.Render("m to show values"))
			}
		}
	}

	help := helpStyle.Render("m show/hide • e edit • c copy • x move • v versions • r rename • d delete • D destroy • esc back • q quit")
	return lipgloss.JoinVertical(lipgloss.Left, title, boxStyle.Render(strings.TrimRight(b.String(), "\n")), help)
}

func lastSegment(p string) string {
	parts := strings.Split(strings.Trim(p, "/"), "/")
	if len(parts) == 0 {
		return p
	}
	return parts[len(parts)-1]
}
