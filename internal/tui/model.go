// Package tui implements the interactive Bubble Tea application: browsing
// KV v2 secret engines, viewing/editing secrets and their version history,
// and renaming or deleting secrets and folders.
package tui

import (
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hashicorp/vault/api"
	vaultpkg "vault-tui/internal/vault"
)

type screen int

const (
	screenMounts screen = iota
	screenBrowse
	screenSecret
	screenNewName
	screenEditor
	screenRename
	screenConfirm
)

// Model is the root Bubble Tea model for the application.
type Model struct {
	client *api.Client
	cfg    vaultpkg.Config

	width, height int

	screen     screen
	prevScreen screen

	err    error
	status string

	// mounts screen
	mounts    []vaultpkg.Mount
	mountList list.Model
	quitting  bool

	// browse screen
	currentMount   string
	currentPath    string
	entries        []vaultpkg.Entry
	entryList      list.Model
	loadingEntries bool
	picker         *pendingOp // armed copy/move waiting for a paste destination

	// secret view screen
	secretPath           string
	secret               *api.KVSecret
	versions             []api.KVVersionMetadata
	viewingVersion       int
	loadingSecret        bool
	showVersions         bool
	versionList          list.Model
	pendingEditAfterLoad bool
	revealSecret         bool

	// new-secret / new-folder name prompt
	newNameInput    textinput.Model
	newNameIsFolder bool

	// key/value editor
	editor editorModel

	// rename prompt
	renameInput    textinput.Model
	renameIsFolder bool
	renameOldPath  string
	renameParent   string // fixed, non-editable — renaming only changes the leaf name

	// confirm prompt
	confirmMessage string
	confirmDanger  bool
	confirmAction  tea.Cmd
	confirmBack    screen
}

// New builds the root model. client must already be authenticated.
func New(client *api.Client, cfg vaultpkg.Config) Model {
	// Compact, single-line-per-item delegate: no description row, no gap
	// between rows.
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)

	mountList := list.New(nil, delegate, 0, 0)
	mountList.Title = "Secret engines (KV)"
	mountList.SetShowStatusBar(false)
	mountList.SetShowHelp(false)
	mountList.Filter = substringFilter

	entryList := list.New(nil, delegate, 0, 0)
	entryList.SetShowStatusBar(false)
	entryList.SetShowHelp(false)
	entryList.Filter = substringFilter

	versionList := list.New(nil, delegate, 0, 0)
	versionList.Title = "Version history"
	versionList.SetShowStatusBar(false)
	versionList.SetShowHelp(false)
	versionList.Filter = substringFilter

	newNameInput := textinput.New()
	newNameInput.Placeholder = "path/of/new-secret"
	newNameInput.CharLimit = 512

	renameInput := textinput.New()
	renameInput.CharLimit = 512

	return Model{
		client:       client,
		cfg:          cfg,
		screen:       screenMounts,
		mountList:    mountList,
		entryList:    entryList,
		versionList:  versionList,
		newNameInput: newNameInput,
		renameInput:  renameInput,
	}
}

func (m Model) Init() tea.Cmd {
	return loadMountsCmd(m.client)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Ctrl+C always quits, no matter which screen or field has focus. This
	// is the one guaranteed way out; 'q' is a context-sensitive shortcut
	// that only fires on navigation screens (see handleKey), since it must
	// remain typeable inside text fields.
	if km, ok := msg.(tea.KeyMsg); ok && km.Type == tea.KeyCtrlC {
		m.quitting = true
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		listW, listH := m.listSize()
		m.mountList.SetSize(listW, listH)
		m.entryList.SetSize(listW, listH)
		m.versionList.SetSize(listW, listH)
		return m, nil

	case mountsLoadedMsg:
		if msg.err != nil {
			log.Printf("loadMounts failed: %v", msg.err)
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.mounts = msg.mounts
		items := make([]list.Item, len(msg.mounts))
		for i, mnt := range msg.mounts {
			items[i] = mountItem{mount: mnt}
		}
		m.mountList.SetItems(items)
		if len(msg.mounts) == 0 {
			m.status = "No KV secret engines found on this Vault server."
		}
		return m, nil

	case entriesLoadedMsg:
		m.loadingEntries = false
		if msg.err != nil {
			log.Printf("loadEntries(%s, %s) failed: %v", m.currentMount, m.currentPath, msg.err)
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.entries = msg.entries
		items := make([]list.Item, len(msg.entries))
		for i, e := range msg.entries {
			items[i] = entryItem{entry: e}
		}
		m.entryList.SetItems(items)
		return m, nil

	case secretLoadedMsg:
		m.loadingSecret = false
		if msg.err != nil {
			log.Printf("loadSecret(%s) failed: %v", m.secretPath, msg.err)
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.secret = msg.secret
		m.versions = msg.versions
		m.revealSecret = false // always land on a freshly loaded/switched secret masked
		items := make([]list.Item, len(msg.versions))
		for i, v := range msg.versions {
			items[i] = versionItem{v: v}
		}
		m.versionList.SetItems(items)

		if m.pendingEditAfterLoad {
			m.pendingEditAfterLoad = false
			var data map[string]interface{}
			if m.secret != nil {
				data = m.secret.Data
			}
			m.editor = newEditorModel(m.secretPath, data)
			m.prevScreen = screenSecret
			m.screen = screenEditor
		}
		return m, nil

	case secretWrittenMsg:
		if msg.err != nil {
			log.Printf("writeSecret(%s) failed: %v", msg.path, msg.err)
			m.err = msg.err
			m.status = ""
			return m, nil
		}
		m.err = nil
		m.status = statusOKStyle.Render("Saved " + msg.path + " as a new version.")
		m.screen = screenSecret
		m.secretPath = msg.path
		m.loadingSecret = true
		return m, tea.Batch(loadSecretCmd(m.client, m.currentMount, msg.path, 0), loadEntriesCmd(m.client, m.currentMount, m.currentPath))

	case secretDeletedMsg:
		if msg.err != nil {
			log.Printf("deleteSecret(%s) failed: %v", msg.path, msg.err)
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.status = statusOKStyle.Render("Deleted " + msg.path + ".")
		m.screen = screenBrowse
		m.loadingEntries = true
		return m, loadEntriesCmd(m.client, m.currentMount, m.currentPath)

	case renamedMsg:
		if msg.err != nil {
			log.Printf("rename (%s) failed: %v", msg.kind, msg.err)
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.status = statusOKStyle.Render("Renamed successfully.")
		m.screen = screenBrowse
		m.loadingEntries = true
		return m, loadEntriesCmd(m.client, m.currentMount, m.currentPath)

	case folderCreatedMsg:
		if msg.err != nil {
			log.Printf("createFolder(%s) failed: %v", msg.path, msg.err)
			m.err = msg.err
			m.status = ""
			return m, nil
		}
		m.err = nil
		m.status = statusOKStyle.Render("Created folder " + msg.path + "/.")
		m.screen = screenBrowse
		m.loadingEntries = true
		return m, loadEntriesCmd(m.client, m.currentMount, m.currentPath)

	case pasteCheckMsg:
		if msg.err != nil {
			log.Printf("pasteCheck(%s/%s) failed: %v", msg.dstMount, msg.dstPath, msg.err)
			m.err = msg.err
			m.status = ""
			return m, nil
		}
		m.err = nil
		if msg.exists {
			verb := "Copy"
			if msg.op.kind == "move" {
				verb = "Move"
			}
			target := "the existing secret"
			if msg.op.isFolder {
				target = "secrets already inside that folder (any with matching names)"
			}
			m.confirmMessage = verb + " may overwrite " + target + " at \"" + msg.dstMount + "/" + msg.dstPath + "\"\n(creates a new version there — the old data isn't lost, just superseded). Continue?"
			m.confirmDanger = true
			m.confirmAction = pasteCmd(m.client, msg.op, msg.dstMount, msg.dstPath)
			m.confirmBack = screenBrowse
			m.prevScreen = screenBrowse
			m.screen = screenConfirm
			return m, nil
		}
		m.status = statusWarnStyle.Render("Pasting...")
		return m, pasteCmd(m.client, msg.op, msg.dstMount, msg.dstPath)

	case pasteDoneMsg:
		if msg.err != nil {
			log.Printf("paste(%s -> %s/%s) failed: %v", msg.op.srcPath, msg.dstMount, msg.dstPath, msg.err)
			m.err = msg.err
			m.status = ""
			return m, nil
		}
		m.err = nil
		verb := "Copied"
		if msg.op.kind == "move" {
			verb = "Moved"
		}
		m.status = statusOKStyle.Render(verb + " to " + msg.dstMount + "/" + msg.dstPath + ".")
		m.picker = nil
		m.screen = screenBrowse
		m.loadingEntries = true
		return m, loadEntriesCmd(m.client, m.currentMount, m.currentPath)

	case tea.KeyMsg:
		log.Printf("key=%q screen=%d", msg.String(), m.screen)
		return m.handleKey(msg)

	case list.FilterMatchesMsg:
		log.Printf("filter matched %d item(s)", len(msg))
	}

	// Anything else (list.FilterMatchesMsg, textinput/list cursor blink
	// ticks, spinner ticks, ...) belongs to whatever sub-component is
	// currently active. Bubble Tea components drive their own animations
	// and async work — like the debounced filter match results — through
	// messages like these; if we don't route them back in, the filter
	// input keeps accepting keystrokes but the actual filtered results
	// never get applied, so the list looks like it's matching everything.
	return m.forwardToActive(msg)
}

func (m Model) forwardToActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.screen {
	case screenMounts:
		m.mountList, cmd = m.mountList.Update(msg)
	case screenBrowse:
		m.entryList, cmd = m.entryList.Update(msg)
	case screenSecret:
		if m.showVersions {
			m.versionList, cmd = m.versionList.Update(msg)
		}
	case screenNewName:
		m.newNameInput, cmd = m.newNameInput.Update(msg)
	case screenRename:
		m.renameInput, cmd = m.renameInput.Update(msg)
	case screenEditor:
		if len(m.editor.rows) > 0 {
			row := &m.editor.rows[m.editor.focusRow]
			if m.editor.focusCol == 0 {
				row.key, cmd = row.key.Update(msg)
			} else {
				row.val, cmd = row.val.Update(msg)
			}
		}
	}
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenMounts:
		return m.updateMounts(msg)
	case screenBrowse:
		return m.updateBrowse(msg)
	case screenSecret:
		return m.updateSecret(msg)
	case screenNewName:
		return m.updateNewName(msg)
	case screenEditor:
		return m.updateEditor(msg)
	case screenRename:
		return m.updateRename(msg)
	case screenConfirm:
		return m.updateConfirm(msg)
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var body string
	switch m.screen {
	case screenMounts:
		body = m.viewMounts()
	case screenBrowse:
		body = m.viewBrowse()
	case screenSecret:
		body = m.viewSecret()
	case screenNewName:
		body = m.viewNewName()
	case screenEditor:
		body = m.viewEditor()
	case screenRename:
		body = m.viewRename()
	case screenConfirm:
		body = m.viewConfirm()
	}

	var footer string
	if m.err != nil {
		footer = statusErrStyle.Render("Error: " + m.err.Error())
	} else if m.status != "" {
		footer = m.status
	}

	return body + "\n" + footer
}

// listSize computes the (width, height) budget available for a list.Model,
// leaving room for title/breadcrumb and the footer line.
func (m Model) listSize() (int, int) {
	w := m.width - 2
	h := m.height - 4
	if w < 20 {
		w = 20
	}
	if h < 5 {
		h = 5
	}
	return w, h
}

func (m Model) breadcrumb() string {
	if m.currentMount == "" {
		return breadcrumbStyle.Render("mounts")
	}
	parts := []string{m.currentMount}
	if m.currentPath != "" {
		parts = append(parts, strings.Split(m.currentPath, "/")...)
	}
	return breadcrumbStyle.Render(strings.Join(parts, " › "))
}

func joinPath(a, b string) string {
	a = strings.Trim(a, "/")
	b = strings.Trim(b, "/")
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "/" + b
}

func parentPath(p string) string {
	p = strings.Trim(p, "/")
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return ""
	}
	return p[:idx]
}
