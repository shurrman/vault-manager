package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vault-manager/internal/domain"
)

type fakeAuthBackend struct {
	fakeBackend
	address    string
	session    domain.Session
	sessionErr error
	loginErr   error
	request    domain.LoginRequest
	logins     int
}

func (f *fakeAuthBackend) Address() string {
	return f.address
}

func (f *fakeAuthBackend) Session(context.Context) (domain.Session, error) {
	return f.session, f.sessionErr
}

func (f *fakeAuthBackend) Login(_ context.Context, request domain.LoginRequest) (domain.Session, error) {
	f.logins++
	f.request = request
	return f.session, f.loginErr
}

func TestAuthenticationFailureAutomaticallyOpensLogin(t *testing.T) {
	backend := &fakeAuthBackend{
		address:    "http://vault.example:8400",
		sessionErr: &domain.AuthenticationError{Err: errors.New("authenticate: Vault returned HTTP 403")},
	}
	model := New(backend, "never")
	model.width = 100
	model.height = 30
	msg := model.Init()().(sessionLoadedMsg)
	_, cmd := model.handleSessionLoaded(msg)

	if model.mode != modeLogin || !model.login.automatic || cmd == nil {
		t.Fatalf("automatic login = mode %v automatic=%v cmd=%v", model.mode, model.login.automatic, cmd)
	}
	model.handleLoginKey(tea.KeyPressMsg{Code: tea.KeyTab})
	view := model.renderLogin()
	if !strings.Contains(view, "http://vault.example:8400") || !strings.Contains(view, "userpass") || !strings.Contains(view, "token") {
		t.Fatalf("login dialog = %q", view)
	}
}

func TestNonAuthenticationFailureDoesNotOpenLogin(t *testing.T) {
	backend := &fakeAuthBackend{
		address:    "http://vault.example:8400",
		sessionErr: errors.New("authenticate: request failed"),
	}
	model := New(backend, "never")
	msg := model.Init()().(sessionLoadedMsg)
	model.handleSessionLoaded(msg)
	if model.mode == modeLogin || model.sessionCheckErr == "" {
		t.Fatalf("network failure opened login: mode=%v error=%q", model.mode, model.sessionCheckErr)
	}
}

func TestLoginKeyOpensEditableURLAndMethodDropdown(t *testing.T) {
	backend := &fakeAuthBackend{address: "http://vault.example:8400"}
	model := New(backend, "never")
	model.width = 100
	model.height = 30
	_, cmd := model.handleKey("l")
	if model.mode != modeLogin || cmd == nil || model.login.address.Value() != backend.address {
		t.Fatalf("manual login = mode %v address %q cmd=%v", model.mode, model.login.address.Value(), cmd)
	}
	model.login.address.SetValue("http://edited.example:8400")
	model.handleLoginKey(tea.KeyPressMsg{Code: tea.KeyTab})
	model.handleLoginKey(tea.KeyPressMsg{Code: tea.KeyDown})
	if model.login.method != domain.AuthToken {
		t.Fatalf("selected method = %q", model.login.method)
	}
	view := model.renderLogin()
	if !strings.Contains(view, "http://edited.example:8400") || !strings.Contains(view, "Token") || strings.Contains(view, "Password") {
		t.Fatalf("token dialog = %q", view)
	}
}

func TestTokenLoginRedactsSecretAndShowsAuthenticatedUser(t *testing.T) {
	backend := &fakeAuthBackend{
		address: "http://vault.example:8400",
		session: domain.Session{Address: "http://edited.example:8400", DisplayName: "alice"},
	}
	model := New(backend, "never")
	model.width = 100
	model.height = 30
	model.openLogin(false)
	model.login.address.SetValue("http://edited.example:8400")
	model.login.method = domain.AuthToken
	model.login.field = loginIdentityField
	model.login.secret.SetValue("token-that-must-not-render")
	if view := model.renderLogin(); strings.Contains(view, "token-that-must-not-render") {
		t.Fatal("token is visible in login dialog")
	}

	_, cmd := model.submitLogin()
	if cmd == nil || model.mode != modeLoggingIn || model.login.secret.Value() != "" {
		t.Fatalf("token submit = mode %v cmd=%v secret retained=%v", model.mode, cmd, model.login.secret.Value() != "")
	}
	msg := cmd().(loginFinishedMsg)
	if backend.request.Token != "token-that-must-not-render" || strings.Contains(loginDebugValue(backend.request), backend.request.Token) {
		t.Fatal("token request was lost or exposed by debug rendering")
	}
	_, refresh := model.handleLoginFinished(msg)
	if refresh == nil || model.mode != modeBrowse || model.sessionUser != "alice" {
		t.Fatalf("login success = mode %v user %q refresh=%v", model.mode, model.sessionUser, refresh)
	}
	if header := model.render(); !strings.Contains(header, "Authenticated as: alice") {
		t.Fatalf("authenticated user missing from header: %q", header)
	}
}

func TestUserpassLoginRedactsPasswordAndKeepsDialogOnFailure(t *testing.T) {
	backend := &fakeAuthBackend{
		address:  "http://vault.example:8400",
		loginErr: errors.New("login: Vault returned HTTP 400"),
	}
	model := New(backend, "never")
	model.width = 100
	model.height = 30
	model.openLogin(false)
	model.login.username.SetValue("alice")
	model.login.secret.SetValue("password-that-must-not-render")
	model.login.field = loginPasswordField
	if view := model.renderLogin(); strings.Contains(view, "password-that-must-not-render") {
		t.Fatal("password is visible in login dialog")
	}

	_, cmd := model.submitLogin()
	msg := cmd().(loginFinishedMsg)
	model.handleLoginFinished(msg)
	if model.mode != modeLogin || model.login.secret.Value() != "" || model.login.err == "" {
		t.Fatalf("failed userpass login = mode %v secret retained=%v error=%q", model.mode, model.login.secret.Value() != "", model.login.err)
	}
	if backend.request.Username != "alice" || backend.request.Password != "password-that-must-not-render" {
		t.Fatal("userpass request was not passed to backend")
	}
}

func TestLoginDialogUsesConfiguredLanguage(t *testing.T) {
	backend := &fakeAuthBackend{address: "http://vault.example:8400"}
	model := New(backend, "never", "ru")
	model.width = 100
	model.height = 30
	model.openLogin(false)
	model.login.field = loginMethodField
	view := model.renderLogin()
	for _, expected := range []string{"Вход в Vault", "URL Vault", "Метод аутентификации", "Пароль"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("translated login dialog lacks %q: %q", expected, view)
		}
	}
}
