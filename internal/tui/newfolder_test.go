package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	vaultpkg "vault-tui/internal/vault"
)

func TestNewFolderFlow(t *testing.T) {
	m := New(nil, vaultpkg.Config{})
	m.screen = screenBrowse
	m.currentMount = "secret"
	m.currentPath = "team"

	tm, cmd := tea.Model(m).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	got := tm.(Model)
	if got.screen != screenNewName {
		t.Fatalf("expected screen=screenNewName after 'N', got %v", got.screen)
	}
	if !got.newNameIsFolder {
		t.Fatalf("expected newNameIsFolder=true after 'N'")
	}
	if cmd != nil {
		t.Fatalf("expected no command from pressing 'N', got one")
	}

	tm = typeString(t, tea.Model(got), "backend")
	tm, cmd = tm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = tm.(Model)

	if got.screen != screenBrowse {
		t.Fatalf("expected screen=screenBrowse after confirming new folder, got %v", got.screen)
	}
	if cmd == nil {
		t.Fatalf("expected createFolderCmd to be returned")
	}
}

func TestNewSecretFlowStillDefaultsToNotFolder(t *testing.T) {
	m := New(nil, vaultpkg.Config{})
	m.screen = screenBrowse
	m.currentMount = "secret"
	m.newNameIsFolder = true // simulate a stale value from a previous 'N'

	tm, _ := tea.Model(m).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got := tm.(Model)
	if got.newNameIsFolder {
		t.Fatalf("pressing 'n' after a previous 'N' must reset newNameIsFolder to false")
	}

	tm = typeString(t, tea.Model(got), "myapp")
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = tm.(Model)
	if got.screen != screenEditor {
		t.Fatalf("expected screen=screenEditor for a plain new secret, got %v", got.screen)
	}
}
