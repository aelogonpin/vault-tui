package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	vaultpkg "vault-tui/internal/vault"
)

// TestRenamePromptLocksParentPath is a regression guard for a real
// incident: renaming used to pre-fill the input with the *full* path, so
// editing it (even by accident) could drop or change the parent folder and
// move/collide the secret somewhere else entirely. The rename prompt must
// only ever let you edit the leaf name — the parent comes from where you
// triggered the rename and isn't part of the editable value at all.
func TestRenamePromptLocksParentPath(t *testing.T) {
	m := New(nil, vaultpkg.Config{})
	m.screen = screenBrowse
	m.currentMount = "secret"
	m.currentPath = "prueba"
	m.entryList.SetItems([]list.Item{
		entryItem{entry: vaultpkg.Entry{Name: "secreto"}},
	})

	tm, _ := tea.Model(m).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	got := tm.(Model)

	if got.screen != screenRename {
		t.Fatalf("expected screen=screenRename after 'r', got %v", got.screen)
	}
	if got.renameParent != "prueba" {
		t.Fatalf("expected renameParent=%q, got %q", "prueba", got.renameParent)
	}
	if got.renameOldPath != "prueba/secreto" {
		t.Fatalf("expected renameOldPath=%q, got %q", "prueba/secreto", got.renameOldPath)
	}
	if v := got.renameInput.Value(); v != "secreto" {
		t.Fatalf("expected the editable field to contain only the leaf name %q, got %q (parent must not be editable)", "secreto", v)
	}

	// Confirming with an empty leaf must not fire any command.
	for range got.renameInput.Value() {
		var cmd tea.Cmd
		tm, cmd = tm.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		got = tm.(Model)
		_ = cmd
	}
	tm, cmd := tm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = tm.(Model)
	if got.screen != screenRename {
		t.Fatalf("empty leaf name must not proceed with the rename, stayed on screenRename expected, got %v", got.screen)
	}
	if cmd != nil {
		t.Fatalf("empty leaf name must not return a rename command")
	}
}
