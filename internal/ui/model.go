package ui

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vault-manager/internal/domain"
)

type Pane int

const (
	PaneSections Pane = iota
	PaneItems
	PanePreview
)

type loadState int

const (
	stateIdle loadState = iota
	stateLoading
	stateReady
	stateError
)

type Model struct {
	backend domain.Backend

	width  int
	height int
	color  bool

	pane         Pane
	sectionIndex int
	itemIndex    int
	detailIndex  int

	items        []domain.Item
	preview      domain.Preview
	listState    loadState
	previewState loadState
	listError    string
	previewError string

	listRequestID    uint64
	previewRequestID uint64
	editorRequestID  uint64
	status           string

	mode   appMode
	editor editorState
}

type listLoadedMsg struct {
	requestID uint64
	section   domain.Section
	items     []domain.Item
	err       error
}

type previewLoadedMsg struct {
	requestID uint64
	section   domain.Section
	itemID    string
	preview   domain.Preview
	err       error
}

func New(backend domain.Backend, colorMode string) *Model {
	color := colorMode == "always" || (colorMode == "auto" && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb")
	return &Model{
		backend:      backend,
		color:        color,
		pane:         PaneSections,
		listState:    stateIdle,
		previewState: stateIdle,
		status:       "Connecting to Vault…",
	}
}

func (m *Model) Init() tea.Cmd {
	return m.loadSection()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		return m, nil
	case tea.KeyPressMsg:
		if m.mode != modeBrowse {
			return m.handleEditorKey(typed)
		}
		return m.handleKey(typed.Keystroke())
	case listLoadedMsg:
		return m.handleListLoaded(typed)
	case previewLoadedMsg:
		return m.handlePreviewLoaded(typed)
	case editorLoadedMsg:
		return m.handleEditorLoaded(typed)
	case editorSavedMsg:
		return m.handleEditorSaved(typed)
	}
	return m, nil
}

func (m *Model) View() tea.View {
	content := m.render()
	view := tea.NewView(content)
	view.AltScreen = true
	view.WindowTitle = "vault-manager"
	return view
}

func (m *Model) handleKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "left", "shift+tab":
		return m.movePane(-1, false)
	case "right":
		return m.movePane(1, false)
	case "tab":
		return m.movePane(1, true)
	case "up":
		return m.move(-1)
	case "down":
		return m.move(1)
	case "home":
		return m.moveToBoundary(false)
	case "end":
		return m.moveToBoundary(true)
	case "r":
		if m.pane == PaneSections {
			return m, m.loadSection()
		}
		if len(m.items) > 0 {
			return m, m.loadPreview()
		}
	case "enter":
		return m.activate()
	case "?":
		m.status = "Tab/Shift+Tab or ←→ switch panes · ↑↓ move · Enter activates · r refreshes · q quits"
	}
	return m, nil
}

func (m *Model) movePane(direction int, preferActions bool) (tea.Model, tea.Cmd) {
	if direction < 0 {
		if m.pane > PaneSections {
			m.pane--
		}
		m.status = ""
		return m, nil
	}

	if m.pane == PaneSections && m.listState == stateReady && len(m.items) > 0 {
		m.pane = PaneItems
	} else if m.pane == PaneItems && m.previewState == stateReady {
		m.pane = PanePreview
		if preferActions {
			m.detailIndex = 0
			if len(m.preview.Actions) > 0 {
				m.detailIndex = len(m.preview.Rows)
			}
		}
	}
	m.status = ""
	return m, nil
}

func (m *Model) move(delta int) (tea.Model, tea.Cmd) {
	switch m.pane {
	case PaneSections:
		next := clamp(m.sectionIndex+delta, 0, len(domain.Sections)-1)
		if next == m.sectionIndex {
			return m, nil
		}
		m.sectionIndex = next
		m.itemIndex = 0
		m.detailIndex = 0
		return m, m.loadSection()
	case PaneItems:
		if len(m.items) == 0 {
			return m, nil
		}
		next := clamp(m.itemIndex+delta, 0, len(m.items)-1)
		if next == m.itemIndex {
			return m, nil
		}
		m.itemIndex = next
		m.detailIndex = 0
		return m, m.loadPreview()
	case PanePreview:
		count := len(m.preview.Rows) + len(m.preview.Actions)
		if count > 0 {
			m.detailIndex = clamp(m.detailIndex+delta, 0, count-1)
		}
	}
	return m, nil
}

func (m *Model) moveToBoundary(last bool) (tea.Model, tea.Cmd) {
	delta := -1
	if last {
		delta = 1
	}
	switch m.pane {
	case PaneSections:
		index := 0
		if last {
			index = len(domain.Sections) - 1
		}
		if index != m.sectionIndex {
			m.sectionIndex = index
			m.itemIndex = 0
			m.detailIndex = 0
			return m, m.loadSection()
		}
	case PaneItems:
		if len(m.items) > 0 {
			index := 0
			if last {
				index = len(m.items) - 1
			}
			if index != m.itemIndex {
				m.itemIndex = index
				m.detailIndex = 0
				return m, m.loadPreview()
			}
		}
	case PanePreview:
		count := len(m.preview.Rows) + len(m.preview.Actions)
		if count > 0 {
			if delta < 0 {
				m.detailIndex = 0
			} else {
				m.detailIndex = count - 1
			}
		}
	}
	return m, nil
}

func (m *Model) activate() (tea.Model, tea.Cmd) {
	if m.pane == PaneSections {
		if m.listState == stateReady && len(m.items) > 0 {
			m.pane = PaneItems
		}
		return m, nil
	}
	if m.pane == PaneItems {
		if m.previewState == stateReady {
			m.pane = PanePreview
		}
		return m, nil
	}
	if m.previewState != stateReady {
		return m, nil
	}
	rowCount := len(m.preview.Rows)
	if m.detailIndex < rowCount {
		row := m.preview.Rows[m.detailIndex]
		if row.Editable {
			m.status = "Editor will open here"
		} else {
			m.status = "Read-only property"
		}
		return m, nil
	}
	actionIndex := m.detailIndex - rowCount
	if actionIndex < 0 || actionIndex >= len(m.preview.Actions) {
		return m, nil
	}
	action := m.preview.Actions[actionIndex]
	if !action.Enabled {
		m.status = action.Label + ": " + action.Reason
		return m, nil
	}
	return m, m.openEditor(action)
}

func (m *Model) handleListLoaded(msg listLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.requestID != m.listRequestID || msg.section != m.section() {
		return m, nil
	}
	if msg.err != nil {
		m.listState = stateError
		m.listError = msg.err.Error()
		m.status = "Section failed to load; press r to retry"
		return m, nil
	}
	m.items = msg.items
	m.listState = stateReady
	m.listError = ""
	m.itemIndex = clamp(m.itemIndex, 0, max(0, len(m.items)-1))
	if len(m.items) == 0 {
		m.preview = domain.Preview{}
		m.previewState = stateIdle
		m.status = "No items"
		return m, nil
	}
	m.status = fmt.Sprintf("Loaded %d %s", len(m.items), strings.ToLower(msg.section.Label()))
	return m, m.loadPreview()
}

func (m *Model) handlePreviewLoaded(msg previewLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.requestID != m.previewRequestID || msg.section != m.section() || len(m.items) == 0 || msg.itemID != m.items[m.itemIndex].ID {
		return m, nil
	}
	if msg.err != nil {
		m.previewState = stateError
		m.previewError = msg.err.Error()
		m.status = "Preview failed to load; press r to retry"
		return m, nil
	}
	m.preview = msg.preview
	m.previewState = stateReady
	m.previewError = ""
	m.detailIndex = clamp(m.detailIndex, 0, max(0, len(m.preview.Rows)+len(m.preview.Actions)-1))
	return m, nil
}

func (m *Model) loadSection() tea.Cmd {
	m.listRequestID++
	requestID := m.listRequestID
	section := m.section()
	m.listState = stateLoading
	m.previewState = stateIdle
	m.items = nil
	m.preview = domain.Preview{}
	m.listError = ""
	m.previewError = ""
	m.status = "Loading " + section.Label() + "…"
	backend := m.backend
	return func() tea.Msg {
		items, err := backend.List(context.Background(), section)
		return listLoadedMsg{requestID: requestID, section: section, items: items, err: err}
	}
}

func (m *Model) loadPreview() tea.Cmd {
	if len(m.items) == 0 {
		return nil
	}
	m.previewRequestID++
	requestID := m.previewRequestID
	section := m.section()
	item := m.items[m.itemIndex]
	m.previewState = stateLoading
	m.preview = domain.Preview{}
	m.previewError = ""
	backend := m.backend
	return func() tea.Msg {
		preview, err := backend.Preview(context.Background(), section, item)
		return previewLoadedMsg{requestID: requestID, section: section, itemID: item.ID, preview: preview, err: err}
	}
}

func (m *Model) section() domain.Section {
	return domain.Sections[m.sectionIndex]
}

func (m *Model) render() string {
	if m.width == 0 || m.height == 0 {
		return "Starting vault-manager…"
	}
	if m.width < 80 || m.height < 20 {
		return fmt.Sprintf("vault-manager needs at least 80x20; current terminal is %dx%d", m.width, m.height)
	}

	headerStyle := lipgloss.NewStyle().Bold(true)
	if m.color {
		headerStyle = headerStyle.Foreground(lipgloss.Color("6"))
	}
	header := headerStyle.Render("vault-manager") + "  " + m.section().Label()

	bodyHeight := m.height - 2
	leftWidth := max(18, m.width*22/100)
	middleWidth := max(26, m.width*31/100)
	rightWidth := m.width - leftWidth - middleWidth

	left := m.renderPane("Sections", m.sectionLines(), leftWidth, bodyHeight, m.pane == PaneSections)
	middle := m.renderPane(m.section().Label(), m.itemLines(), middleWidth, bodyHeight, m.pane == PaneItems)
	right := m.renderPane(m.editorTitle(), m.previewLines(), rightWidth, bodyHeight, m.pane == PanePreview)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, middle, right)

	footer := m.footerText()
	if m.status != "" {
		footer = m.status + "  ·  " + footer
	}
	footer = truncate(footer, m.width)
	return header + "\n" + body + "\n" + footer
}

func (m *Model) renderPane(title string, lines []string, width, height int, active bool) string {
	borderColor := lipgloss.Color("8")
	titleStyle := lipgloss.NewStyle().Bold(true)
	if m.color {
		if active {
			borderColor = lipgloss.Color("12")
			titleStyle = titleStyle.Foreground(lipgloss.Color("12"))
		} else {
			titleStyle = titleStyle.Foreground(lipgloss.Color("6"))
		}
	}
	contentHeight := max(1, height-3)
	if len(lines) > contentHeight {
		lines = lines[:contentHeight]
	}
	content := titleStyle.Render(title) + "\n" + strings.Join(lines, "\n")
	style := lipgloss.NewStyle().Width(max(1, width-2)).Height(max(1, height-2)).Border(lipgloss.RoundedBorder())
	if m.color {
		style = style.BorderForeground(borderColor)
	}
	return style.Render(content)
}

func (m *Model) sectionLines() []string {
	lines := make([]string, 0, len(domain.Sections))
	for index, section := range domain.Sections {
		lines = append(lines, m.renderSelection(section.Label(), index == m.sectionIndex, m.pane == PaneSections && index == m.sectionIndex))
	}
	return lines
}

func (m *Model) itemLines() []string {
	switch m.listState {
	case stateLoading:
		return []string{"[loading…]"}
	case stateError:
		return []string{m.listError, "", "Press r to retry"}
	case stateReady:
		if len(m.items) == 0 {
			return []string{"[no items]"}
		}
		lines := make([]string, 0, len(m.items))
		for index, item := range m.items {
			label := item.Name
			if item.Subtitle != "" {
				label += "  " + item.Subtitle
			}
			lines = append(lines, m.renderSelection(label, index == m.itemIndex, m.pane == PaneItems && index == m.itemIndex))
		}
		return visibleWindow(lines, m.itemIndex, max(1, m.height-5))
	default:
		return []string{"[not loaded]"}
	}
}

func (m *Model) previewLines() []string {
	if m.mode != modeBrowse {
		return m.editorLines()
	}
	switch m.previewState {
	case stateLoading:
		return []string{"[loading…]"}
	case stateError:
		return []string{m.previewError, "", "Press r to retry"}
	case stateReady:
		lines := make([]string, 0, len(m.preview.Rows)+len(m.preview.Actions)+2)
		selectedLine := 0
		if m.preview.Title != "" {
			lines = append(lines, m.preview.Title)
		}
		if m.preview.Subtitle != "" {
			lines = append(lines, m.preview.Subtitle)
		}
		for index, row := range m.preview.Rows {
			label := row.Label + ": " + strings.ReplaceAll(row.Value, "\n", " · ")
			if index == m.detailIndex {
				selectedLine = len(lines)
			}
			lines = append(lines, m.renderSelection(label, index == m.detailIndex, m.pane == PanePreview && index == m.detailIndex))
		}
		if len(m.preview.Actions) > 0 {
			lines = append(lines, "", "Actions")
		}
		for actionIndex, action := range m.preview.Actions {
			index := len(m.preview.Rows) + actionIndex
			label := action.Label
			if !action.Enabled {
				label = "[disabled] " + label
			}
			if index == m.detailIndex {
				selectedLine = len(lines)
			}
			lines = append(lines, m.renderSelection(label, index == m.detailIndex, m.pane == PanePreview && index == m.detailIndex))
		}
		return visibleWindow(lines, selectedLine, max(1, m.height-5))
	default:
		return []string{"[nothing selected]"}
	}
}

func (m *Model) renderSelection(text string, selected, active bool) string {
	prefix := "  "
	style := lipgloss.NewStyle()
	if m.color {
		if active {
			style = style.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("4")).Bold(true).Reverse(true)
		} else if selected {
			style = style.Foreground(lipgloss.Color("14")).Bold(true)
		}
	} else if active {
		prefix = "> "
	} else if selected {
		prefix = "* "
	}
	return style.Render(prefix + text)
}

func visibleWindow(lines []string, selected, capacity int) []string {
	if capacity <= 0 || len(lines) <= capacity {
		return lines
	}
	start := 0
	if selected >= capacity {
		start = selected - capacity + 1
	}
	if start+capacity > len(lines) {
		start = len(lines) - capacity
	}
	return lines[start : start+capacity]
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func truncate(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return string(runes[:max(0, width)])
	}
	return string(runes[:width-1]) + "…"
}
