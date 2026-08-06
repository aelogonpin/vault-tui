package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hashicorp/vault/api"
	vaultpkg "vault-tui/internal/vault"
)

const opTimeout = 30 * time.Second

func withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), opTimeout)
}

type mountsLoadedMsg struct {
	mounts []vaultpkg.Mount
	err    error
}

type entriesLoadedMsg struct {
	entries []vaultpkg.Entry
	err     error
}

type secretLoadedMsg struct {
	secret   *api.KVSecret
	versions []api.KVVersionMetadata
	err      error
}

type secretWrittenMsg struct {
	path string
	err  error
}

type secretDeletedMsg struct {
	path string
	err  error
}

type renamedMsg struct {
	kind string // "secret" or "folder"
	err  error
}

type folderCreatedMsg struct {
	path string
	err  error
}

// pendingOp is an armed copy/move waiting for a destination to be picked
// by browsing (possibly into a different mount entirely).
type pendingOp struct {
	kind     string // "copy" or "move"
	srcMount string
	srcPath  string
	isFolder bool // srcPath is a folder (recursive) rather than a single secret
}

type pasteCheckMsg struct {
	op       pendingOp
	dstMount string
	dstPath  string
	exists   bool
	err      error
}

type pasteDoneMsg struct {
	op       pendingOp
	dstMount string
	dstPath  string
	err      error
}

func loadMountsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := withTimeout()
		defer cancel()
		mounts, err := vaultpkg.ListKVMounts(ctx, client)
		return mountsLoadedMsg{mounts: mounts, err: err}
	}
}

func loadEntriesCmd(client *api.Client, mount, path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := withTimeout()
		defer cancel()
		entries, err := vaultpkg.ListPath(ctx, client, mount, path)
		return entriesLoadedMsg{entries: entries, err: err}
	}
}

func loadSecretCmd(client *api.Client, mount, path string, version int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := withTimeout()
		defer cancel()
		secret, err := vaultpkg.ReadSecret(ctx, client, mount, path, version)
		if err != nil {
			return secretLoadedMsg{err: err}
		}
		versions, err := vaultpkg.ListVersions(ctx, client, mount, path)
		return secretLoadedMsg{secret: secret, versions: versions, err: err}
	}
}

func writeSecretCmd(client *api.Client, mount, path string, data map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := withTimeout()
		defer cancel()
		_, err := vaultpkg.WriteSecret(ctx, client, mount, path, data)
		return secretWrittenMsg{path: path, err: err}
	}
}

func deleteSecretCmd(client *api.Client, mount, path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := withTimeout()
		defer cancel()
		err := vaultpkg.DeleteLatestVersion(ctx, client, mount, path)
		return secretDeletedMsg{path: path, err: err}
	}
}

func destroySecretCmd(client *api.Client, mount, path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := withTimeout()
		defer cancel()
		err := vaultpkg.DeleteMetadata(ctx, client, mount, path)
		return secretDeletedMsg{path: path, err: err}
	}
}

func renameSecretCmd(client *api.Client, mount, oldPath, newPath string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := withTimeout()
		defer cancel()
		err := vaultpkg.RenameSecret(ctx, client, mount, oldPath, newPath)
		return renamedMsg{kind: "secret", err: err}
	}
}

func createFolderCmd(client *api.Client, mount, path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := withTimeout()
		defer cancel()
		err := vaultpkg.CreateFolder(ctx, client, mount, path)
		return folderCreatedMsg{path: path, err: err}
	}
}

func pasteCheckCmd(client *api.Client, op pendingOp, dstMount, dstPath string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := withTimeout()
		defer cancel()
		var exists bool
		var err error
		if op.isFolder {
			exists, err = vaultpkg.FolderHasContent(ctx, client, dstMount, dstPath)
		} else {
			exists, err = vaultpkg.SecretExists(ctx, client, dstMount, dstPath)
		}
		return pasteCheckMsg{op: op, dstMount: dstMount, dstPath: dstPath, exists: exists, err: err}
	}
}

func pasteCmd(client *api.Client, op pendingOp, dstMount, dstPath string) tea.Cmd {
	return func() tea.Msg {
		timeout := opTimeout
		if op.isFolder {
			timeout = 2 * time.Minute // may touch many secrets, same budget as renameFolderCmd
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		var err error
		switch {
		case op.isFolder && op.kind == "move":
			err = vaultpkg.MoveFolder(ctx, client, op.srcMount, op.srcPath, dstMount, dstPath)
		case op.isFolder:
			err = vaultpkg.CopyFolder(ctx, client, op.srcMount, op.srcPath, dstMount, dstPath)
		case op.kind == "move":
			err = vaultpkg.MoveSecret(ctx, client, op.srcMount, op.srcPath, dstMount, dstPath)
		default:
			err = vaultpkg.CopySecret(ctx, client, op.srcMount, op.srcPath, dstMount, dstPath)
		}
		return pasteDoneMsg{op: op, dstMount: dstMount, dstPath: dstPath, err: err}
	}
}

func renameFolderCmd(client *api.Client, mount, oldPrefix, newPrefix string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		err := vaultpkg.RenameFolder(ctx, client, mount, oldPrefix, newPrefix)
		return renamedMsg{kind: "folder", err: err}
	}
}
