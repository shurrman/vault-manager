package ui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vault-manager/internal/config"
	"vault-manager/internal/i18n"
)

type setupStep int

const (
	setupLanguage setupStep = iota
	setupAddress
	setupToken
	setupDone
)

var setupLanguages = []struct {
	code string
	name string
}{
	{code: "en", name: "English"},
	{code: "ru", name: "Русский"},
	{code: "es", name: "Español"},
	{code: "zh", name: "简体中文"},
}

type SetupModel struct {
	config        config.Config
	text          i18n.Translator
	step          setupStep
	languageIndex int
	input         textinput.Model
	err           string
	cancelled     bool
}

func NewSetup(cfg config.Config, askLanguage bool) *SetupModel {
	input := textinput.New()
	input.Prompt = "> "
	input.SetWidth(68)
	model := &SetupModel{config: cfg, text: i18n.New(cfg.UI.Language), input: input}
	for index, language := range setupLanguages {
		if language.code == cfg.UI.Language {
			model.languageIndex = index
			break
		}
	}
	if askLanguage {
		model.step = setupLanguage
	} else {
		model.advance()
	}
	return model
}

func (m *SetupModel) Init() tea.Cmd {
	if m.step == setupAddress || m.step == setupToken {
		return m.input.Focus()
	}
	return nil
}

func (m *SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if ok {
		switch key.Keystroke() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		}
		if m.step == setupLanguage {
			return m.updateLanguage(key.Keystroke())
		}
		if key.Keystroke() == "enter" {
			return m.acceptInput()
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *SetupModel) View() tea.View {
	lines := []string{lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render(m.text.Text("Initial configuration")), ""}
	switch m.step {
	case setupLanguage:
		lines = append(lines, m.text.Text("Select interface language"), "")
		for index, language := range setupLanguages {
			prefix := "  "
			if index == m.languageIndex {
				prefix = "> "
			}
			lines = append(lines, prefix+language.name)
		}
		lines = append(lines, "", m.text.Text("↑↓ choose  Enter select  Ctrl+C cancel"))
	case setupAddress:
		lines = append(lines, m.text.Text("Enter Vault URL"), "", m.input.View(), "", m.text.Text("Enter continue  Ctrl+C cancel"))
	case setupToken:
		lines = append(lines, m.text.Text("Enter Vault token"), "", m.input.View(), "", m.text.Text("Enter continue  Ctrl+C cancel"))
	}
	if m.err != "" {
		lines = append(lines, "", lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9")).Render("! "+m.err))
	}
	content := lipgloss.NewStyle().Width(76).Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("12")).Render(strings.Join(lines, "\n"))
	view := tea.NewView(content)
	view.AltScreen = true
	view.WindowTitle = "vault-manager setup"
	return view
}

func (m *SetupModel) Config() config.Config {
	return m.config
}

func (m *SetupModel) Cancelled() bool {
	return m.cancelled
}

func (m *SetupModel) updateLanguage(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up":
		if m.languageIndex > 0 {
			m.languageIndex--
		}
	case "down":
		if m.languageIndex < len(setupLanguages)-1 {
			m.languageIndex++
		}
	case "home":
		m.languageIndex = 0
	case "end":
		m.languageIndex = len(setupLanguages) - 1
	case "enter":
		m.config.UI.Language = setupLanguages[m.languageIndex].code
		m.text = i18n.New(m.config.UI.Language)
		return m, m.advance()
	}
	return m, nil
}

func (m *SetupModel) acceptInput() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	switch m.step {
	case setupAddress:
		candidate := m.config
		candidate.Vault.Address = value
		if err := candidate.ValidatePartial(); err != nil {
			m.err = m.text.Text(err.Error())
			return m, nil
		}
		m.config.Vault.Address = value
	case setupToken:
		m.config.Vault.Token = value
		if err := m.config.Validate(); err != nil {
			m.err = m.text.Text(err.Error())
			return m, nil
		}
	}
	m.err = ""
	return m, m.advance()
}

func (m *SetupModel) advance() tea.Cmd {
	m.input.Reset()
	if strings.TrimSpace(m.config.Vault.Address) == "" {
		m.step = setupAddress
		m.input.EchoMode = textinput.EchoNormal
		m.input.Placeholder = "http://vault.wsoft:8400"
		return m.input.Focus()
	}
	if strings.TrimSpace(m.config.Vault.Token) == "" {
		m.step = setupToken
		m.input.EchoMode = textinput.EchoPassword
		m.input.Placeholder = ""
		return m.input.Focus()
	}
	m.step = setupDone
	return tea.Quit
}
