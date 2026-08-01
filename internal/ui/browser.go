package ui

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vault-manager/internal/domain"
)

type browserState struct {
	item  domain.Item
	page  domain.BrowserPage
	path  string
	read  bool
	index int
	err   string
}

type browserLoadedMsg struct {
	requestID uint64
	section   domain.Section
	itemID    string
	page      domain.BrowserPage
	err       error
}

func (m *Model) openBrowser() tea.Cmd {
	if _, ok := m.backend.(domain.BrowserBackend); !ok {
		m.status = "Browser backend is unavailable"
		return nil
	}
	if len(m.items) == 0 {
		return nil
	}
	m.browser = browserState{item: m.items[m.itemIndex]}
	return m.loadBrowser("", false)
}

func (m *Model) loadBrowser(path string, read bool) tea.Cmd {
	backend, ok := m.backend.(domain.BrowserBackend)
	if !ok {
		return nil
	}
	m.browserRequestID++
	requestID := m.browserRequestID
	section := m.section()
	item := m.browser.item
	m.mode = modeBrowserLoading
	m.browser.path = path
	m.browser.read = read
	m.browser.err = ""
	m.status = "Loading values…"
	return func() tea.Msg {
		page, err := backend.Browse(context.Background(), section, item, path, read)
		return browserLoadedMsg{requestID: requestID, section: section, itemID: item.ID, page: page, err: err}
	}
}

func (m *Model) handleBrowserLoaded(msg browserLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.requestID != m.browserRequestID || msg.section != m.section() || msg.itemID != m.browser.item.ID {
		return m, nil
	}
	m.mode = modeBrowser
	if msg.err != nil {
		m.browser.err = msg.err.Error()
		m.status = "Values failed to load; press r to retry"
		return m, nil
	}
	m.browser.page = msg.page
	m.browser.index = clamp(m.browser.index, 0, max(0, len(msg.page.Entries)-1))
	m.browser.err = ""
	m.status = "Values loaded"
	return m, nil
}

func (m *Model) handleBrowserKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Keystroke()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.mode == modeBrowserLoading {
		return m, nil
	}
	switch key {
	case "up":
		m.browser.index = clamp(m.browser.index-1, 0, max(0, len(m.browser.page.Entries)-1))
	case "down":
		m.browser.index = clamp(m.browser.index+1, 0, max(0, len(m.browser.page.Entries)-1))
	case "home":
		m.browser.index = 0
	case "end":
		m.browser.index = max(0, len(m.browser.page.Entries)-1)
	case "enter":
		if m.browser.page.Leaf || len(m.browser.page.Entries) == 0 {
			return m, nil
		}
		entry := m.browser.page.Entries[m.browser.index]
		return m, m.loadBrowser(entry.Path, !entry.Directory)
	case "esc", "left":
		if m.browser.path == "" {
			return m.closeBrowser()
		}
		return m, m.loadBrowser(parentUIBrowserPath(m.browser.path), false)
	case "q":
		return m.closeBrowser()
	case "r":
		return m, m.loadBrowser(m.browser.path, m.browser.read)
	}
	return m, nil
}

func (m *Model) closeBrowser() (tea.Model, tea.Cmd) {
	m.mode = modeBrowse
	m.browser = browserState{}
	m.status = "Browser closed"
	return m, nil
}

func (m *Model) browserLines() []string {
	if m.mode == modeBrowserLoading {
		return []string{m.text.Text("[loading…]")}
	}
	if m.browser.err != "" {
		return []string{m.browser.err, "", m.text.Text("Press r to retry")}
	}
	page := m.browser.page
	if page.Leaf {
		lines := make([]string, 0, len(page.Rows))
		for _, row := range page.Rows {
			lines = append(lines, m.text.Text(row.Label)+": "+row.Value)
		}
		return lines
	}
	if len(page.Entries) == 0 {
		return []string{m.text.Text("[no values]")}
	}
	lines := make([]string, 0, len(page.Entries))
	for index, entry := range page.Entries {
		label := entry.Name
		if entry.Directory {
			label += "/"
		}
		lines = append(lines, m.renderSelection(label, index == m.browser.index, index == m.browser.index))
	}
	return visibleWindow(lines, m.browser.index, max(1, m.height-5))
}

func (m *Model) browserTitle() string {
	if m.browser.item.Name != "" {
		mount := strings.TrimSuffix(m.browser.item.Name, "/") + "/"
		if m.browser.path != "" {
			return mount + m.browser.path
		}
		if m.browser.page.DisplayPath != "" {
			return m.browser.page.DisplayPath
		}
		return mount
	}
	return m.text.Text("Browse values")
}

func parentUIBrowserPath(value string) string {
	value = strings.Trim(value, "/")
	if index := strings.LastIndex(value, "/"); index >= 0 {
		return value[:index]
	}
	return ""
}

func (m *Model) renderRowSelection(text string, selected, active bool, severity domain.Severity) string {
	if severity != domain.SeverityDanger {
		return m.renderSelection(text, selected, active)
	}
	prefix := "! "
	style := lipgloss.NewStyle()
	if m.color {
		prefix = "  "
		style = style.Foreground(lipgloss.Color("9")).Bold(true)
		if active {
			style = style.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("1")).Reverse(true)
		}
	} else if selected && !active {
		prefix = "* "
	}
	return style.Render(prefix + text)
}
