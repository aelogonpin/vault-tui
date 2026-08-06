package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	vaultpkg "vault-tui/internal/vault"
)

// applyCmd drives a single tea.Cmd (and, if it's a batch, its immediate
// sub-commands) back through Update, mimicking one tick of what the real
// Bubble Tea runtime does. This is what was missing in the app itself:
// list.FilterMatchesMsg (and other async results) need to be fed back into
// Update to actually take effect.
func applyCmd(t *testing.T, m tea.Model, cmd tea.Cmd) tea.Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			mm := c()
			if mm == nil {
				continue
			}
			m, _ = m.Update(mm)
		}
		return m
	}
	m, _ = m.Update(msg)
	return m
}

func typeString(t *testing.T, m tea.Model, s string) tea.Model {
	t.Helper()
	for _, r := range s {
		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = applyCmd(t, m, cmd)
	}
	return m
}

func TestBrowseFilterNarrowsBySubstring(t *testing.T) {
	m := New(nil, vaultpkg.Config{})
	m.screen = screenBrowse
	m.currentMount = "secret"
	m.entryList.SetSize(80, 20)

	names := []string{"delfos-prod", "delfos-staging", "delfos-dev", "otro-secreto", "algomas"}
	items := make([]list.Item, len(names))
	for i, n := range names {
		items[i] = entryItem{entry: vaultpkg.Entry{Name: n}}
	}
	m.entryList.SetItems(items)

	tm := typeString(t, tea.Model(m), "/delf")

	got := tm.(Model)
	if got.entryList.FilterState() != list.Filtering {
		t.Fatalf("expected list to be in Filtering state, got %v", got.entryList.FilterState())
	}

	visible := got.entryList.VisibleItems()
	if len(visible) != 3 {
		var gotNames []string
		for _, it := range visible {
			gotNames = append(gotNames, it.(entryItem).entry.Name)
		}
		t.Fatalf(`typing "/delf" should match the 3 "delfos-*" entries, got %d: %v`, len(visible), gotNames)
	}
	for _, it := range visible {
		name := it.(entryItem).entry.Name
		if name != "delfos-prod" && name != "delfos-staging" && name != "delfos-dev" {
			t.Fatalf("unexpected item in filtered results: %q", name)
		}
	}
}

func TestBrowseFilterLettersDontTriggerShortcuts(t *testing.T) {
	// Regression test: while filtering, keys that double as list shortcuts
	// (q=quit, h=up-a-level, n=new secret...) must be typed into the filter
	// box, not intercepted as commands.
	m := New(nil, vaultpkg.Config{})
	m.screen = screenBrowse
	m.currentMount = "secret"
	m.currentPath = "some/folder"
	m.entryList.SetSize(80, 20)
	m.entryList.SetItems([]list.Item{
		entryItem{entry: vaultpkg.Entry{Name: "quiet-service"}},
		entryItem{entry: vaultpkg.Entry{Name: "other"}},
	})

	tm := typeString(t, tea.Model(m), "/q")

	got := tm.(Model)
	if got.quitting {
		t.Fatalf("typing 'q' while filtering must not quit the app")
	}
	if got.currentPath != "some/folder" {
		t.Fatalf("typing 'h'-less filter text must not navigate; currentPath changed to %q", got.currentPath)
	}
	if got.entryList.FilterInput.Value() != "q" {
		t.Fatalf("expected filter input value %q, got %q", "q", got.entryList.FilterInput.Value())
	}
}
