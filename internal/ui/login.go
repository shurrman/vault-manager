package ui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vault-manager/internal/domain"
)

const (
	loginAddressField = iota
	loginMethodField
	loginIdentityField
	loginPasswordField
)

type loginState struct {
	address   textinput.Model
	username  textinput.Model
	secret    textinput.Model
	method    domain.AuthMethod
	field     int
	err       string
	automatic bool
}

type sessionLoadedMsg struct {
	requestID uint64
	session   domain.Session
	err       error
}

type loginFinishedMsg struct {
	requestID uint64
	session   domain.Session
	err       error
}

func (m *Model) checkSession() tea.Cmd {
	backend, ok := m.backend.(domain.AuthBackend)
	if !ok {
		return m.loadSection()
	}
	m.authRequestID++
	requestID := m.authRequestID
	m.sessionCheckErr = ""
	m.listState = stateLoading
	m.status = "Checking authentication…"
	return func() tea.Msg {
		session, err := backend.Session(context.Background())
		return sessionLoadedMsg{requestID: requestID, session: session, err: err}
	}
}

func (m *Model) handleSessionLoaded(msg sessionLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.requestID != m.authRequestID {
		return m, nil
	}
	if msg.err != nil {
		m.sessionCheckErr = msg.err.Error()
		m.listState = stateError
		m.listError = msg.err.Error()
		if domain.IsAuthenticationError(msg.err) {
			m.status = "Authentication required"
			return m, m.openLogin(true)
		}
		m.status = "Authentication check failed; press r to retry or l to login"
		return m, nil
	}
	m.sessionUser = msg.session.DisplayName
	m.sessionCheckErr = ""
	return m, m.loadSection()
}

func (m *Model) openLogin(automatic bool) tea.Cmd {
	backend, ok := m.backend.(domain.AuthBackend)
	if !ok {
		m.status = "Login backend is unavailable"
		return nil
	}
	m.listRequestID++
	m.previewRequestID++
	m.editorRequestID++
	m.browserRequestID++
	m.authRequestID++
	m.mode = modeLogin
	m.login = newLoginState(backend.Address(), automatic)
	m.resizeLoginInputs()
	m.status = "Enter Vault credentials"
	return m.focusLoginField()
}

func newLoginState(address string, automatic bool) loginState {
	addressInput := textinput.New()
	addressInput.Prompt = ""
	addressInput.SetValue(address)
	usernameInput := textinput.New()
	usernameInput.Prompt = ""
	secretInput := textinput.New()
	secretInput.Prompt = ""
	secretInput.EchoMode = textinput.EchoPassword
	secretInput.EchoCharacter = '•'
	return loginState{
		address:   addressInput,
		username:  usernameInput,
		secret:    secretInput,
		method:    domain.AuthUserpass,
		automatic: automatic,
	}
}

func (m *Model) resizeLoginInputs() {
	width := max(20, min(68, m.width-12))
	m.login.address.SetWidth(width)
	m.login.username.SetWidth(width)
	m.login.secret.SetWidth(width)
}

func (m *Model) handleLoginKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Keystroke()
	if key == "ctrl+c" {
		m.clearLogin()
		return m, tea.Quit
	}
	if m.mode == modeLoggingIn {
		return m, nil
	}
	switch key {
	case "esc":
		automatic := m.login.automatic
		m.clearLogin()
		m.mode = modeBrowse
		if automatic && m.sessionUser == "" {
			m.listState = stateError
			m.listError = "Authentication required"
			m.status = "Authentication required; press l to login"
		} else {
			m.status = "Login canceled"
		}
		return m, nil
	case "ctrl+s":
		return m.submitLogin()
	case "tab":
		return m.moveLoginField(1)
	case "shift+tab":
		return m.moveLoginField(-1)
	case "enter":
		if m.login.field == m.loginFieldCount()-1 {
			return m.submitLogin()
		}
		return m.moveLoginField(1)
	}

	if m.login.field == loginMethodField {
		switch key {
		case "up", "down", "left", "right":
			if m.login.method == domain.AuthUserpass {
				m.login.method = domain.AuthToken
			} else {
				m.login.method = domain.AuthUserpass
			}
			m.login.secret.Reset()
			m.login.err = ""
		}
		return m, nil
	}

	var cmd tea.Cmd
	switch m.login.field {
	case loginAddressField:
		m.login.address, cmd = m.login.address.Update(msg)
	case loginIdentityField:
		if m.login.method == domain.AuthUserpass {
			m.login.username, cmd = m.login.username.Update(msg)
		} else {
			m.login.secret, cmd = m.login.secret.Update(msg)
		}
	case loginPasswordField:
		m.login.secret, cmd = m.login.secret.Update(msg)
	}
	m.login.err = ""
	return m, cmd
}

func (m *Model) moveLoginField(delta int) (tea.Model, tea.Cmd) {
	count := m.loginFieldCount()
	m.login.field = (m.login.field + delta + count) % count
	return m, m.focusLoginField()
}

func (m *Model) focusLoginField() tea.Cmd {
	m.login.address.Blur()
	m.login.username.Blur()
	m.login.secret.Blur()
	switch m.login.field {
	case loginAddressField:
		return m.login.address.Focus()
	case loginIdentityField:
		if m.login.method == domain.AuthUserpass {
			return m.login.username.Focus()
		}
		return m.login.secret.Focus()
	case loginPasswordField:
		return m.login.secret.Focus()
	}
	return nil
}

func (m *Model) loginFieldCount() int {
	if m.login.method == domain.AuthUserpass {
		return 4
	}
	return 3
}

func (m *Model) submitLogin() (tea.Model, tea.Cmd) {
	backend, ok := m.backend.(domain.AuthBackend)
	if !ok {
		m.login.err = "Login backend is unavailable"
		return m, nil
	}
	request := domain.LoginRequest{
		Address: m.login.address.Value(),
		Method:  m.login.method,
	}
	if m.login.method == domain.AuthUserpass {
		request.Username = m.login.username.Value()
		request.Password = m.login.secret.Value()
	} else {
		request.Token = m.login.secret.Value()
	}
	m.login.secret.Reset()
	m.authRequestID++
	requestID := m.authRequestID
	m.mode = modeLoggingIn
	m.login.err = ""
	m.status = "Authenticating…"
	return m, func() tea.Msg {
		session, err := backend.Login(context.Background(), request)
		return loginFinishedMsg{requestID: requestID, session: session, err: err}
	}
}

func (m *Model) handleLoginFinished(msg loginFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.requestID != m.authRequestID {
		return m, nil
	}
	if msg.err != nil {
		m.mode = modeLogin
		m.login.err = m.text.Text(msg.err.Error())
		m.login.field = m.loginFieldCount() - 1
		m.status = "Login failed"
		return m, m.focusLoginField()
	}
	m.sessionUser = msg.session.DisplayName
	m.sessionCheckErr = ""
	m.clearLogin()
	m.mode = modeBrowse
	m.items = nil
	m.preview = domain.Preview{}
	m.listState = stateIdle
	m.previewState = stateIdle
	cmd := m.loadSection()
	m.status = m.text.Format("Authenticated as %s; refreshing…", msg.session.DisplayName)
	return m, cmd
}

func (m *Model) clearLogin() {
	m.login.address.Reset()
	m.login.username.Reset()
	m.login.secret.Reset()
	m.login = loginState{}
}

func (m *Model) renderLogin() string {
	if m.width == 0 || m.height == 0 {
		return m.text.Text("Starting vault-manager…")
	}
	if m.width < 80 || m.height < 20 {
		return m.text.Format("vault-manager needs at least 80x20; current terminal is %dx%d", m.width, m.height)
	}

	lines := []string{m.text.Text("Login to Vault"), ""}
	lines = append(lines, m.loginInputLine("Vault URL", m.login.address, loginAddressField, false))
	lines = append(lines, m.text.Text("Authentication method")+":")
	if m.login.field == loginMethodField {
		for _, method := range []domain.AuthMethod{domain.AuthUserpass, domain.AuthToken} {
			lines = append(lines, m.renderSelection(string(method), method == m.login.method, method == m.login.method))
		}
	} else {
		lines = append(lines, "  "+string(m.login.method))
	}
	if m.login.method == domain.AuthUserpass {
		lines = append(lines,
			m.loginInputLine("Username", m.login.username, loginIdentityField, false),
			m.loginInputLine("Password", m.login.secret, loginPasswordField, true),
		)
	} else {
		lines = append(lines, m.loginInputLine("Token", m.login.secret, loginIdentityField, true))
	}
	if m.mode == modeLoggingIn {
		lines = append(lines, "", m.text.Text("Authenticating and saving credentials…"))
	}
	if m.login.err != "" {
		errorStyle := lipgloss.NewStyle().Bold(true)
		if m.color {
			errorStyle = errorStyle.Foreground(lipgloss.Color("9"))
		}
		lines = append(lines, "", errorStyle.Render("! "+m.login.err))
	}
	lines = append(lines, "", m.footerText())
	width := min(76, m.width-4)
	style := lipgloss.NewStyle().Width(width).Padding(1, 2).Border(lipgloss.RoundedBorder())
	if m.color {
		style = style.BorderForeground(lipgloss.Color("12"))
	}
	return style.Render(strings.Join(lines, "\n"))
}

func (m *Model) loginInputLine(label string, input textinput.Model, field int, secret bool) string {
	value := input.Value()
	if secret {
		if value == "" {
			value = "[empty]"
		} else {
			value = "[set]"
		}
	}
	if m.login.field == field && m.mode == modeLogin {
		value = input.View()
	}
	return m.renderSelection(m.text.Text(label)+": "+value, m.login.field == field, m.login.field == field)
}

func loginDebugValue(request domain.LoginRequest) string {
	return fmt.Sprintf("address=%s method=%s username=%s password=[redacted] token=[redacted]",
		request.Address, request.Method, request.Username)
}
