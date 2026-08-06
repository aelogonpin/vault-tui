package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hashicorp/vault/api"
	vaultpkg "vault-tui/internal/vault"
)

func TestSecretValuesMaskedByDefaultAndToggle(t *testing.T) {
	m := New(nil, vaultpkg.Config{})
	m.screen = screenSecret
	m.currentMount = "secret"
	m.secretPath = "app/db"

	tm, _ := tea.Model(m).Update(secretLoadedMsg{
		secret: &api.KVSecret{Data: map[string]interface{}{"password": "hunter2"}},
	})
	got := tm.(Model)
	if got.revealSecret {
		t.Fatalf("expected values to be masked right after loading a secret")
	}
	if view := got.viewSecret(); !strings.Contains(view, maskedValueMarker) || strings.Contains(view, "hunter2") {
		t.Fatalf("expected masked view to hide the real value; view=%q", view)
	}

	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	got = tm.(Model)
	if !got.revealSecret {
		t.Fatalf("expected 'm' to reveal values")
	}
	if view := got.viewSecret(); !strings.Contains(view, "hunter2") {
		t.Fatalf("expected revealed view to show the real value; view=%q", view)
	}

	// Loading a different (or the same) secret again must re-mask.
	tm, _ = tm.Update(secretLoadedMsg{
		secret: &api.KVSecret{Data: map[string]interface{}{"password": "hunter2"}},
	})
	got = tm.(Model)
	if got.revealSecret {
		t.Fatalf("expected reveal state to reset to masked on every secret load")
	}
}
