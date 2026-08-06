package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	vaultpkg "vault-tui/internal/vault"
)

func TestArmCopyThenCancel(t *testing.T) {
	m := New(nil, vaultpkg.Config{})
	m.screen = screenBrowse
	m.currentMount = "secret"
	m.currentPath = "prueba"
	m.entryList.SetItems([]list.Item{
		entryItem{entry: vaultpkg.Entry{Name: "secreto"}},
	})

	tm, _ := tea.Model(m).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got := tm.(Model)
	if got.picker == nil {
		t.Fatalf("expected picker to be armed after 'c'")
	}
	if got.picker.kind != "copy" || got.picker.srcMount != "secret" || got.picker.srcPath != "prueba/secreto" {
		t.Fatalf("unexpected picker state: %#v", got.picker)
	}

	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = tm.(Model)
	if got.picker != nil {
		t.Fatalf("expected 'esc' to cancel the armed picker")
	}
}

func TestArmMoveThenPasteAtSourceIsRefused(t *testing.T) {
	// Regression guard: pasting a *move* onto its own source path would
	// copy the data (as a no-op new version) and then immediately delete
	// that same path as "cleanup", destroying the secret entirely. This
	// must be refused before it ever reaches Vault.
	m := New(nil, vaultpkg.Config{})
	m.screen = screenBrowse
	m.currentMount = "secret"
	m.currentPath = "prueba"
	m.entryList.SetItems([]list.Item{
		entryItem{entry: vaultpkg.Entry{Name: "secreto"}},
	})

	tm, _ := tea.Model(m).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	tm, cmd := tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	got := tm.(Model)

	if cmd != nil {
		t.Fatalf("pasting onto the source path must not issue any command")
	}
	if got.picker == nil {
		t.Fatalf("the armed move must stay armed so the user can pick a real destination")
	}
}

func TestArmCopyOnFolderSetsIsFolder(t *testing.T) {
	m := New(nil, vaultpkg.Config{})
	m.screen = screenBrowse
	m.currentMount = "secret"
	m.currentPath = "prueba"
	m.entryList.SetItems([]list.Item{
		entryItem{entry: vaultpkg.Entry{Name: "subcarpeta", IsFolder: true}},
	})

	tm, _ := tea.Model(m).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got := tm.(Model)
	if got.picker == nil {
		t.Fatalf("expected picker to be armed after 'c' on a folder")
	}
	if !got.picker.isFolder || got.picker.srcPath != "prueba/subcarpeta" {
		t.Fatalf("unexpected picker state: %#v", got.picker)
	}
}

func TestArmMoveFolderThenPasteInsideItselfIsRefused(t *testing.T) {
	// Same self-destruction shape as TestArmMoveThenPasteAtSourceIsRefused,
	// but one level removed: pasting a folder into one of its own
	// subfolders would land the copy inside the exact subtree MoveFolder is
	// about to delete once every secret has been copied, wiping it too.
	m := New(nil, vaultpkg.Config{})
	m.screen = screenBrowse
	m.currentMount = "secret"
	m.currentPath = "prueba"
	m.entryList.SetItems([]list.Item{
		entryItem{entry: vaultpkg.Entry{Name: "carpeta", IsFolder: true}},
	})

	// Arm a move on "prueba/carpeta".
	tm, _ := tea.Model(m).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := tm.(Model)
	// Simulate having navigated inside the very folder just armed.
	got.currentPath = "prueba/carpeta/sub"
	tm, cmd := tea.Model(got).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	got = tm.(Model)

	if cmd != nil {
		t.Fatalf("pasting a folder inside its own subtree must not issue any command")
	}
	if got.picker == nil {
		t.Fatalf("the armed move must stay armed so the user can pick a real destination")
	}
}

func TestPasteAtDifferentFolderProceeds(t *testing.T) {
	m := New(nil, vaultpkg.Config{})
	m.screen = screenBrowse
	m.currentMount = "secret"
	m.currentPath = "prueba"
	m.entryList.SetItems([]list.Item{
		entryItem{entry: vaultpkg.Entry{Name: "secreto"}},
	})

	tm, _ := tea.Model(m).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got := tm.(Model)
	got.currentPath = "otra-carpeta" // simulate having navigated elsewhere
	tm, cmd := tea.Model(got).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	got = tm.(Model)

	if cmd == nil {
		t.Fatalf("pasting at a different folder must issue the paste-check command")
	}
	if got.picker == nil {
		t.Fatalf("picker should still be armed until the paste actually completes")
	}
}
