package tui

import (
	"fmt"

	"github.com/hashicorp/vault/api"
	vaultpkg "vault-tui/internal/vault"
)

type mountItem struct {
	mount vaultpkg.Mount
}

func (i mountItem) Title() string {
	if i.mount.Version != "2" {
		return fmt.Sprintf("%s  (kv v%s — unsupported)", i.mount.Path, i.mount.Version)
	}
	return fmt.Sprintf("%s  (kv v2)", i.mount.Path)
}
func (i mountItem) Description() string { return "" }
func (i mountItem) FilterValue() string { return i.mount.Path }

type entryItem struct {
	entry vaultpkg.Entry
}

func (i entryItem) Title() string {
	if i.entry.IsFolder {
		return "📁 " + i.entry.Name + "/"
	}
	if i.entry.Name == vaultpkg.KeepFileName {
		return "🧷 " + i.entry.Name + "  (placeholder — safe to delete)"
	}
	return "🔑 " + i.entry.Name
}
func (i entryItem) Description() string {
	if i.entry.IsFolder {
		return "folder"
	}
	return "secret"
}
func (i entryItem) FilterValue() string { return i.entry.Name }

type versionItem struct {
	v api.KVVersionMetadata
}

func (i versionItem) Title() string {
	label := fmt.Sprintf("v%d  %s", i.v.Version, i.v.CreatedTime.Local().Format("2006-01-02 15:04:05"))
	if i.v.Destroyed {
		label += "  (destroyed)"
	} else if !i.v.DeletionTime.IsZero() {
		label += "  (deleted)"
	}
	return label
}
func (i versionItem) Description() string { return "" }
func (i versionItem) FilterValue() string { return i.Title() }
