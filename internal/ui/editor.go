package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"vault-manager/internal/domain"
)

type appMode int

const (
	modeBrowse appMode = iota
	modeEditorLoading
	modeEditing
	modeConfirm
	modeSaving
)

type editorState struct {
	spec         domain.Editor
	fieldIndex   int
	input        textinput.Model
	area         textarea.Model
	inputActive  bool
	confirmIndex int
}

type editorLoadedMsg struct {
	requestID uint64
	section   domain.Section
	itemID    string
	editor    domain.Editor
	err       error
}

type editorSavedMsg struct {
	requestID uint64
	section   domain.Section
	itemID    string
	preview   domain.Preview
	err       error
}

func (m *Model) openEditor(action domain.Action) tea.Cmd {
	backend, ok := m.backend.(domain.EditorBackend)
	if !ok {
		m.status = "Editor backend is unavailable"
		return nil
	}
	if len(m.items) == 0 {
		return nil
	}
	m.editorRequestID++
	requestID := m.editorRequestID
	section := m.section()
	item := m.items[m.itemIndex]
	m.mode = modeEditorLoading
	m.status = "Opening " + action.Label + "…"
	return func() tea.Msg {
		editor, err := backend.LoadEditor(context.Background(), section, item, action.ID)
		return editorLoadedMsg{requestID: requestID, section: section, itemID: item.ID, editor: editor, err: err}
	}
}

func (m *Model) handleEditorLoaded(msg editorLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.requestID != m.editorRequestID || msg.section != m.section() || len(m.items) == 0 || msg.itemID != m.items[m.itemIndex].ID {
		return m, nil
	}
	if msg.err != nil {
		m.mode = modeBrowse
		m.status = "Editor unavailable: " + msg.err.Error()
		return m, nil
	}
	m.editor = editorState{spec: msg.editor}
	if msg.editor.SubmitAlways && len(msg.editor.Fields) == 0 {
		m.mode = modeConfirm
		m.editor.confirmIndex = 1
		m.status = "Confirmation required"
		return m, nil
	}
	m.mode = modeEditing
	m.status = "Editor opened; Enter edits the selected field; Ctrl+S reviews"
	return m, nil
}

func (m *Model) handleEditorSaved(msg editorSavedMsg) (tea.Model, tea.Cmd) {
	if msg.requestID != m.editorRequestID || msg.section != m.section() || len(m.items) == 0 || msg.itemID != m.items[m.itemIndex].ID {
		return m, nil
	}
	if msg.err != nil {
		var partial *domain.PartialSaveError
		if errors.As(msg.err, &partial) {
			if msg.preview.Title != "" {
				m.preview = msg.preview
				m.previewState = stateReady
			}
			m.mode = modeBrowse
			m.editor = editorState{}
			m.status = partial.Error()
			return m, nil
		}
		m.mode = modeEditing
		m.blurEditorInput()
		m.status = "Save failed: " + msg.err.Error()
		return m, nil
	}
	refreshList := m.editor.spec.RefreshList
	successMessage := m.editor.spec.SuccessMessage
	if successMessage == "" {
		successMessage = "Saved and verified"
	}
	m.preview = msg.preview
	m.previewState = stateReady
	m.mode = modeBrowse
	m.editor = editorState{}
	m.detailIndex = 0
	m.status = successMessage
	if refreshList {
		cmd := m.loadSection()
		m.status = successMessage + "; refreshing…"
		return m, cmd
	}
	return m, nil
}

func (m *Model) handleEditorKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Keystroke()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.mode {
	case modeEditorLoading, modeSaving:
		return m, nil
	case modeConfirm:
		return m.handleConfirmKey(key)
	case modeEditing:
		return m.handleEditingKey(msg)
	default:
		return m.handleKey(key)
	}
}

func (m *Model) handleEditingKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Keystroke()
	if m.editor.inputActive {
		field := m.editor.spec.Fields[m.editor.fieldIndex]
		if field.Kind == domain.FieldMultiline {
			switch key {
			case "esc":
				m.syncInput()
				m.blurEditorInput()
				m.status = "Draft updated"
				return m, nil
			case "ctrl+s":
				m.syncInput()
				m.blurEditorInput()
				return m.reviewEditor()
			default:
				var cmd tea.Cmd
				m.editor.area, cmd = m.editor.area.Update(msg)
				m.syncInput()
				return m, cmd
			}
		}
		switch key {
		case "enter":
			m.syncInput()
			m.blurEditorInput()
			m.status = "Draft updated"
			return m, nil
		case "esc":
			m.syncInput()
			m.blurEditorInput()
			return m.leaveEditor(key == "q")
		case "ctrl+s":
			m.syncInput()
			m.blurEditorInput()
			return m.reviewEditor()
		default:
			var cmd tea.Cmd
			m.editor.input, cmd = m.editor.input.Update(msg)
			m.syncInput()
			return m, cmd
		}
	}

	switch key {
	case "up":
		m.editor.fieldIndex = clamp(m.editor.fieldIndex-1, 0, max(0, len(m.editor.spec.Fields)-1))
	case "down":
		m.editor.fieldIndex = clamp(m.editor.fieldIndex+1, 0, max(0, len(m.editor.spec.Fields)-1))
	case "home":
		m.editor.fieldIndex = 0
	case "end":
		m.editor.fieldIndex = max(0, len(m.editor.spec.Fields)-1)
	case "enter":
		return m.startFieldEdit()
	case " ":
		if m.toggleBoolean() {
			m.status = "Draft updated"
		}
	case "ctrl+s":
		return m.reviewEditor()
	case "esc":
		return m.leaveEditor(false)
	case "q":
		return m.leaveEditor(true)
	}
	return m, nil
}

func (m *Model) startFieldEdit() (tea.Model, tea.Cmd) {
	if len(m.editor.spec.Fields) == 0 {
		return m, nil
	}
	field := &m.editor.spec.Fields[m.editor.fieldIndex]
	switch field.Kind {
	case domain.FieldReadOnly:
		m.status = field.Label + ": read-only"
		return m, nil
	case domain.FieldBoolean:
		m.toggleBoolean()
		m.status = "Draft updated"
		return m, nil
	case domain.FieldMultiline:
		area := textarea.New()
		area.Prompt = ""
		area.SetWidth(max(20, m.width*47/100-8))
		area.SetHeight(max(6, min(14, m.height/2)))
		area.SetValue(field.Value)
		m.editor.area = area
		m.editor.inputActive = true
		return m, m.editor.area.Focus()
	case domain.FieldText, domain.FieldSecret:
		input := textinput.New()
		input.Prompt = ""
		input.SetWidth(max(20, m.width*47/100-12))
		input.SetValue(field.Value)
		if field.Kind == domain.FieldSecret {
			input.EchoMode = textinput.EchoPassword
			input.EchoCharacter = '•'
		}
		m.editor.input = input
		m.editor.inputActive = true
		return m, m.editor.input.Focus()
	default:
		return m, nil
	}
}

func (m *Model) toggleBoolean() bool {
	if len(m.editor.spec.Fields) == 0 {
		return false
	}
	field := &m.editor.spec.Fields[m.editor.fieldIndex]
	if field.Kind != domain.FieldBoolean {
		return false
	}
	if strings.EqualFold(field.Value, "true") {
		field.Value = "false"
	} else {
		field.Value = "true"
	}
	return true
}

func (m *Model) blurEditorInput() {
	if len(m.editor.spec.Fields) == 0 {
		m.editor.inputActive = false
		return
	}
	if m.editor.spec.Fields[m.editor.fieldIndex].Kind == domain.FieldMultiline {
		m.editor.area.Blur()
	} else {
		m.editor.input.Blur()
	}
	m.editor.inputActive = false
}

func (m *Model) syncInput() {
	if len(m.editor.spec.Fields) == 0 || !m.editor.inputActive {
		return
	}
	field := &m.editor.spec.Fields[m.editor.fieldIndex]
	if field.Kind == domain.FieldMultiline {
		field.Value = m.editor.area.Value()
		return
	}
	field.Value = m.editor.input.Value()
}

func (m *Model) reviewEditor() (tea.Model, tea.Cmd) {
	if m.editor.spec.Dirty() || m.editor.spec.SubmitAlways {
		m.mode = modeConfirm
		m.editor.confirmIndex = len(m.confirmChoices()) - 1
		m.status = "Review action"
	}
	return m, nil
}

func (m *Model) leaveEditor(quit bool) (tea.Model, tea.Cmd) {
	if m.editor.spec.Dirty() || m.editor.spec.SubmitAlways {
		return m.reviewEditor()
	}
	m.mode = modeBrowse
	m.editor = editorState{}
	if quit {
		return m, tea.Quit
	}
	m.status = "Editor closed"
	return m, nil
}

func (m *Model) handleConfirmKey(key string) (tea.Model, tea.Cmd) {
	choiceCount := len(m.confirmChoices())
	switch key {
	case "left":
		m.editor.confirmIndex = (m.editor.confirmIndex + choiceCount - 1) % choiceCount
	case "right":
		m.editor.confirmIndex = (m.editor.confirmIndex + 1) % choiceCount
	case "s":
		m.editor.confirmIndex = 0
		return m.confirmChoice()
	case "d":
		if choiceCount == 3 {
			m.editor.confirmIndex = 1
			return m.confirmChoice()
		}
	case "c", "esc", "q":
		m.editor.confirmIndex = choiceCount - 1
		return m.confirmChoice()
	case "enter":
		return m.confirmChoice()
	}
	return m, nil
}

func (m *Model) confirmChoices() []string {
	submit := m.editor.spec.SubmitLabel
	if submit == "" {
		submit = "Save"
	}
	if m.editor.spec.SubmitAlways && !m.editor.spec.Dirty() {
		return []string{submit, "Cancel"}
	}
	return []string{submit, "Discard", "Cancel"}
}

func (m *Model) confirmChoice() (tea.Model, tea.Cmd) {
	choices := m.confirmChoices()
	if m.editor.confirmIndex == len(choices)-1 {
		if len(choices) == 2 {
			m.mode = modeBrowse
			m.editor = editorState{}
			m.status = "Action canceled"
			return m, nil
		}
		m.mode = modeEditing
		m.status = "Continue editing"
		return m, nil
	}
	switch m.editor.confirmIndex {
	case 0:
		backend, ok := m.backend.(domain.EditorBackend)
		if !ok {
			m.mode = modeEditing
			m.status = "Editor backend is unavailable"
			return m, nil
		}
		m.editorRequestID++
		requestID := m.editorRequestID
		section := m.section()
		item := m.items[m.itemIndex]
		spec := m.editor.spec
		m.mode = modeSaving
		m.status = "Saving…"
		return m, func() tea.Msg {
			preview, err := backend.SaveEditor(context.Background(), spec)
			return editorSavedMsg{requestID: requestID, section: section, itemID: item.ID, preview: preview, err: err}
		}
	case 1:
		m.mode = modeBrowse
		m.editor = editorState{}
		m.status = "Changes discarded"
	}
	return m, nil
}

func (m *Model) editorLines() []string {
	switch m.mode {
	case modeEditorLoading:
		return []string{"[opening editor…]"}
	case modeSaving:
		return []string{"[saving and verifying…]"}
	case modeConfirm:
		return m.confirmLines()
	case modeEditing:
		capacity := max(1, m.height-5)
		lines := []string{m.editor.spec.Title}
		for _, warning := range m.editor.spec.Warnings {
			lines = append(lines, "! "+warning)
		}
		if len(m.editor.spec.Warnings) > 0 {
			lines = append(lines, "")
		}
		selectedLine := 0
		for index, field := range m.editor.spec.Fields {
			value := field.Value
			if field.Kind == domain.FieldMultiline {
				lineCount := strings.Count(value, "\n") + 1
				value = fmt.Sprintf("[%d lines · %d bytes]", lineCount, len(value))
			}
			if field.Kind == domain.FieldSecret {
				if value == "" {
					value = "[empty]"
				} else {
					value = "[set]"
				}
			}
			if m.editor.inputActive && index == m.editor.fieldIndex && field.Kind == domain.FieldMultiline {
				selectedLine = len(lines)
				lines = append(lines, m.renderSelection(field.Label+":", true, true))
				for _, areaLine := range strings.Split(m.editor.area.View(), "\n") {
					lines = append(lines, "  "+areaLine)
				}
				continue
			}
			if m.editor.inputActive && index == m.editor.fieldIndex {
				value = m.editor.input.View()
			}
			if field.Kind == domain.FieldReadOnly {
				value += "  [read-only]"
			}
			line := field.Label + ": " + value
			if index == m.editor.fieldIndex {
				selectedLine = len(lines)
			}
			lines = append(lines, m.renderSelection(line, index == m.editor.fieldIndex, index == m.editor.fieldIndex))
		}
		help := m.editorHelpLines()
		helpLimit := max(3, min(7, capacity/2))
		if len(help) > helpLimit {
			help = append(append([]string(nil), help[:helpLimit-1]...), "  …")
		}
		formCapacity := max(1, capacity-len(help))
		visible := visibleWindow(lines, selectedLine, formCapacity)
		return append(visible, help...)
	default:
		return m.previewLines()
	}
}

func (m *Model) editorHelpLines() []string {
	if len(m.editor.spec.Fields) == 0 {
		return nil
	}
	field := m.editor.spec.Fields[m.editor.fieldIndex]
	lines := []string{"", "Help"}
	if field.Help != "" {
		lines = append(lines, "  "+field.Help)
	}
	for _, example := range field.Examples {
		exampleLines := strings.Split(example, "\n")
		lines = append(lines, "  Example: "+exampleLines[0])
		for _, continuation := range exampleLines[1:] {
			lines = append(lines, "           "+continuation)
		}
	}
	for _, tip := range m.editor.spec.Tips {
		lines = append(lines, "  Tip: "+tip)
	}
	return lines
}

func (m *Model) confirmLines() []string {
	lines := []string{"Review action"}
	for _, warning := range m.editor.spec.Warnings {
		lines = append(lines, "! "+warning)
	}
	if m.editor.spec.Dirty() {
		lines = append(lines, "", "Diff")
		for _, field := range m.editor.spec.Fields {
			if field.Value == field.Original {
				continue
			}
			change := field.Original + " → " + field.Value
			if field.Kind == domain.FieldSecret {
				change = "[secret changed]"
			} else if field.Kind == domain.FieldMultiline {
				change = fmt.Sprintf("[%d bytes] → [%d bytes]", len(field.Original), len(field.Value))
			}
			lines = append(lines, field.Label+": "+change)
		}
	}
	lines = append(lines, "", "Choose an action")
	choices := m.confirmChoices()
	for index, choice := range choices {
		lines = append(lines, m.renderSelection(choice, index == m.editor.confirmIndex, index == m.editor.confirmIndex))
	}
	return visibleWindow(lines, len(lines)-len(choices)+m.editor.confirmIndex, max(1, m.height-5))
}

func (m *Model) editorTitle() string {
	switch m.mode {
	case modeEditorLoading:
		return "Editor"
	case modeEditing:
		return "Editor"
	case modeConfirm:
		return "Confirm"
	case modeSaving:
		return "Saving"
	default:
		return "Preview"
	}
}

func (m *Model) footerText() string {
	switch m.mode {
	case modeEditing:
		if m.editor.inputActive {
			if m.editor.spec.Fields[m.editor.fieldIndex].Kind == domain.FieldMultiline {
				return "Edit text  Enter newline  Esc accept  Ctrl+S review"
			}
			return "Type value  Enter accept  Ctrl+S review  Esc review changes"
		}
		return "↑↓ field  Enter edit/toggle  Space toggle  Ctrl+S/Esc review"
	case modeConfirm:
		return "←→ choose  Enter confirm  s submit  d discard  c/Esc cancel"
	case modeSaving:
		return "Saving and verifying; input is locked"
	case modeEditorLoading:
		return "Opening editor…"
	default:
		return "Tab/Shift+Tab or ←→ pane  ↑↓ move  Enter activate  r refresh  ? help  q quit"
	}
}

func editorDebugValue(editor domain.Editor) string {
	parts := make([]string, 0, len(editor.Fields))
	for _, field := range editor.Fields {
		value := field.Value
		if field.Kind == domain.FieldSecret {
			value = "[redacted]"
		}
		parts = append(parts, fmt.Sprintf("%s=%s", field.ID, value))
	}
	return strings.Join(parts, ",")
}
