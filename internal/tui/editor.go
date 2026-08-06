package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	keyFieldWidth   = 20
	valueFieldWidth = 36
)

type kvRow struct {
	key textinput.Model
	val textinput.Model
}

type editorModel struct {
	targetPath string
	rows       []kvRow
	focusRow   int
	focusCol   int // 0 = key, 1 = value
}

func newKVInput(placeholder string, width int) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 4096
	ti.Width = width
	return ti
}

func newKVRow() kvRow {
	return kvRow{
		key: newKVInput("key", keyFieldWidth),
		val: newKVInput(`text or JSON…`, valueFieldWidth),
	}
}

func newEditorModel(targetPath string, data map[string]interface{}) editorModel {
	e := editorModel{targetPath: targetPath}

	if len(data) == 0 {
		e.rows = []kvRow{newKVRow()}
	} else {
		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			row := newKVRow()
			row.key.SetValue(k)
			row.val.SetValue(toDisplayString(data[k]))
			e.rows = append(e.rows, row)
		}
	}

	e.syncFocus()
	return e
}

// looksLikeJSON reports whether s would itself be reinterpreted as
// something other than a plain string by parseValue.
func looksLikeJSON(s string) bool {
	var v interface{}
	return json.Unmarshal([]byte(s), &v) == nil
}

// toDisplayString renders a stored secret value for editing. Vault KV
// values are usually strings but the API allows any JSON scalar/object;
// non-string values are rendered as their JSON text so they round-trip
// through parseValue exactly. A string value that happens to already look
// like JSON (e.g. a secret whose literal value is "12345" or "true") is
// quoted on display for the same reason — otherwise saving without any
// edits would silently turn it into a number/bool.
func toDisplayString(v interface{}) string {
	if s, ok := v.(string); ok {
		if looksLikeJSON(s) {
			if q, err := json.Marshal(s); err == nil {
				return string(q)
			}
		}
		return s
	}
	if b, err := json.Marshal(v); err == nil {
		return string(b)
	}
	return fmt.Sprintf("%v", v)
}

// parseValue turns editor input back into a Vault secret value. If it
// parses as JSON (a number, bool, null, quoted string, object, or array)
// that's what gets stored; otherwise the raw text is stored as-is. This
// means ordinary text (passwords, tokens, URLs...) round-trips unchanged,
// while typing real JSON — or wrapping a JSON-looking value in "quotes" to
// force it to stay a string — is also honored.
func parseValue(s string) interface{} {
	var decoded interface{}
	if err := json.Unmarshal([]byte(s), &decoded); err == nil {
		return decoded
	}
	return s
}

type valueKind int

const (
	kindText valueKind = iota
	kindJSONString
	kindJSONNumber
	kindJSONBool
	kindJSONNull
	kindJSONObject
	kindJSONArray
)

func classifyValue(s string) valueKind {
	if strings.TrimSpace(s) == "" {
		return kindText
	}
	var decoded interface{}
	if err := json.Unmarshal([]byte(s), &decoded); err != nil {
		return kindText
	}
	switch decoded.(type) {
	case string:
		return kindJSONString
	case float64:
		return kindJSONNumber
	case bool:
		return kindJSONBool
	case nil:
		return kindJSONNull
	case map[string]interface{}:
		return kindJSONObject
	case []interface{}:
		return kindJSONArray
	default:
		return kindText
	}
}

func (k valueKind) tag() string {
	switch k {
	case kindJSONString:
		return "json·string"
	case kindJSONNumber:
		return "json·number"
	case kindJSONBool:
		return "json·bool"
	case kindJSONNull:
		return "json·null"
	case kindJSONObject:
		return "json·object"
	case kindJSONArray:
		return "json·array"
	default:
		return "text"
	}
}

func renderValueTag(s string) string {
	kind := classifyValue(s)
	if kind == kindText {
		return helpStyle.Render(kind.tag())
	}
	return jsonTagStyle.Render(kind.tag())
}

func (e *editorModel) syncFocus() {
	for i := range e.rows {
		e.rows[i].key.Blur()
		e.rows[i].val.Blur()
		e.rows[i].key.TextStyle = blurredFieldStyle
		e.rows[i].val.TextStyle = blurredFieldStyle
	}
	if len(e.rows) == 0 {
		return
	}
	if e.focusRow >= len(e.rows) {
		e.focusRow = len(e.rows) - 1
	}
	if e.focusCol == 0 {
		e.rows[e.focusRow].key.Focus()
		e.rows[e.focusRow].key.TextStyle = focusedFieldStyle
	} else {
		e.rows[e.focusRow].val.Focus()
		e.rows[e.focusRow].val.TextStyle = focusedFieldStyle
	}
}

func (e *editorModel) addRow() {
	e.rows = append(e.rows, newKVRow())
	e.focusRow = len(e.rows) - 1
	e.focusCol = 0
	e.syncFocus()
}

func (e *editorModel) removeCurrentRow() {
	if len(e.rows) <= 1 {
		e.rows = []kvRow{newKVRow()}
		e.focusRow, e.focusCol = 0, 0
		e.syncFocus()
		return
	}
	e.rows = append(e.rows[:e.focusRow], e.rows[e.focusRow+1:]...)
	if e.focusRow >= len(e.rows) {
		e.focusRow = len(e.rows) - 1
	}
	e.syncFocus()
}

func (e editorModel) toData() map[string]interface{} {
	data := make(map[string]interface{}, len(e.rows))
	for _, row := range e.rows {
		k := strings.TrimSpace(row.key.Value())
		if k == "" {
			continue
		}
		data[k] = parseValue(row.val.Value())
	}
	return data
}

func (m Model) updateEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.err = nil
		m.status = ""
		if m.prevScreen == screenSecret && m.secret != nil {
			m.screen = screenSecret
		} else {
			m.screen = screenBrowse
		}
		return m, nil

	case "ctrl+s":
		data := m.editor.toData()
		if len(data) == 0 {
			m.status = statusWarnStyle.Render("Add at least one key/value pair before saving.")
			return m, nil
		}
		m.status = ""
		m.err = nil
		return m, writeSecretCmd(m.client, m.currentMount, m.editor.targetPath, data)

	case "ctrl+n":
		m.editor.addRow()
		return m, nil

	case "ctrl+x":
		m.editor.removeCurrentRow()
		return m, nil

	case "tab":
		if m.editor.focusCol == 0 {
			m.editor.focusCol = 1
		} else if m.editor.focusRow < len(m.editor.rows)-1 {
			m.editor.focusRow++
			m.editor.focusCol = 0
		}
		m.editor.syncFocus()
		return m, nil

	case "shift+tab":
		if m.editor.focusCol == 1 {
			m.editor.focusCol = 0
		} else if m.editor.focusRow > 0 {
			m.editor.focusRow--
			m.editor.focusCol = 1
		}
		m.editor.syncFocus()
		return m, nil

	case "up":
		if m.editor.focusRow > 0 {
			m.editor.focusRow--
			m.editor.syncFocus()
		}
		return m, nil

	case "down":
		if m.editor.focusRow < len(m.editor.rows)-1 {
			m.editor.focusRow++
			m.editor.syncFocus()
		}
		return m, nil
	}

	var cmd tea.Cmd
	row := &m.editor.rows[m.editor.focusRow]
	if m.editor.focusCol == 0 {
		row.key, cmd = row.key.Update(msg)
	} else {
		row.val, cmd = row.val.Update(msg)
	}
	return m, cmd
}

func padTo(s string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(s)
}

func (m Model) viewEditor() string {
	title := titleStyle.Render(" vault-tui ") + "  " + breadcrumbStyle.Render(m.editor.targetPath)

	header := "  " + columnHeaderStyle.Render(padTo("KEY", keyFieldWidth)) + "  " + columnHeaderStyle.Render("VALUE")

	var b strings.Builder
	b.WriteString(header + "\n")
	for i, row := range m.editor.rows {
		marker := "  "
		if i == m.editor.focusRow {
			marker = focusedFieldStyle.Render("▸ ")
		}
		tag := renderValueTag(row.val.Value())
		b.WriteString(marker + row.key.View() + "  " + row.val.View() + "  " + tag + "\n")
	}

	hint := helpStyle.Render(`value: plain text or JSON (42, true, null, {"a":1}, ["x"]) — wrap in "quotes" to force text`)
	help := helpStyle.Render("tab/shift+tab move • ↑/↓ row • ctrl+n add row • ctrl+x remove row • ctrl+s save • esc cancel")
	return lipgloss.JoinVertical(lipgloss.Left, title, boxStyle.Render(strings.TrimRight(b.String(), "\n")), hint, help)
}
