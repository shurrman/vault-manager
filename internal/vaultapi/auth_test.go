package vaultapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"vault-manager/internal/config"
	"vault-manager/internal/domain"
)

type authFixture struct {
	mu               sync.Mutex
	tokens           map[string]map[string]interface{}
	username         string
	password         string
	issuedToken      string
	loginCount       int
	loginTokenHeader string
}

func (f *authFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodPut && r.URL.Path == "/v1/auth/userpass/login/"+f.username:
		f.loginCount++
		f.loginTokenHeader = r.Header.Get("X-Vault-Token")
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || stringValue(payload["password"]) != f.password {
			w.WriteHeader(http.StatusForbidden)
			writeJSON(w, map[string]interface{}{"errors": []string{"invalid credentials"}})
			return
		}
		writeJSON(w, map[string]interface{}{"auth": map[string]interface{}{
			"client_token": f.issuedToken,
			"metadata":     map[string]string{"username": f.username},
		}})
	case r.Method == http.MethodGet && r.URL.Path == "/v1/auth/token/lookup-self":
		data, ok := f.tokens[r.Header.Get("X-Vault-Token")]
		if !ok {
			w.WriteHeader(http.StatusForbidden)
			writeJSON(w, map[string]interface{}{"errors": []string{"permission denied"}})
			return
		}
		writeJSON(w, map[string]interface{}{"data": data})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestSessionReportsConfiguredUserAndAuthenticationFailure(t *testing.T) {
	fixture := &authFixture{tokens: map[string]map[string]interface{}{
		"configured-token": {"display_name": "configured-display", "meta": map[string]interface{}{"username": "configured-user"}},
	}}
	server := httptest.NewServer(fixture)
	defer server.Close()
	path, cfg := authConfig(t, server.URL, "configured-token")
	client, err := New(cfg, path)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	session, err := client.Session(context.Background())
	if err != nil {
		t.Fatalf("Session() error = %v", err)
	}
	if session.DisplayName != "configured-user" || session.Address != server.URL {
		t.Fatalf("session = %#v", session)
	}

	client.api.SetToken("invalid-token")
	if _, err := client.Session(context.Background()); err == nil || !domain.IsAuthenticationError(err) {
		t.Fatalf("invalid Session() error = %v", err)
	}
}

func TestUserpassLoginUsesEditedURLAndPersistsIssuedToken(t *testing.T) {
	oldFixture := &authFixture{tokens: map[string]map[string]interface{}{
		"old-token": {"display_name": "old-user"},
	}}
	oldServer := httptest.NewServer(oldFixture)
	defer oldServer.Close()
	path, cfg := authConfig(t, oldServer.URL, "old-token")
	client, err := New(cfg, path)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	newFixture := &authFixture{
		tokens: map[string]map[string]interface{}{
			"issued-token": {"display_name": "userpass-alice", "meta": map[string]interface{}{"username": "alice"}},
		},
		username:    "alice",
		password:    "test-password",
		issuedToken: "issued-token",
	}
	newServer := httptest.NewServer(newFixture)
	defer newServer.Close()

	session, err := client.Login(context.Background(), domain.LoginRequest{
		Address:  newServer.URL,
		Method:   domain.AuthUserpass,
		Username: "alice",
		Password: "test-password",
	})
	if err != nil {
		t.Fatalf("Login(userpass) error = %v", err)
	}
	if session.DisplayName != "alice" || client.Address() != newServer.URL {
		t.Fatalf("session = %#v address=%q", session, client.Address())
	}
	newFixture.mu.Lock()
	if newFixture.loginCount != 1 || newFixture.loginTokenHeader != "" {
		t.Fatalf("userpass requests = %d old token header = %q", newFixture.loginCount, newFixture.loginTokenHeader)
	}
	newFixture.mu.Unlock()

	saved, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load(saved config) error = %v", err)
	}
	if saved.Vault.Address != newServer.URL || saved.Vault.Token != "issued-token" {
		t.Fatalf("saved credentials = address %q token changed=%v", saved.Vault.Address, saved.Vault.Token == "issued-token")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if strings.Contains(string(data), "test-password") {
		t.Fatal("password was written to config")
	}
}

func TestTokenLoginPersistsOnlyAfterSuccessfulLookup(t *testing.T) {
	fixture := &authFixture{tokens: map[string]map[string]interface{}{
		"old-token": {"display_name": "old-user"},
		"new-token": {"display_name": "new-user"},
	}}
	server := httptest.NewServer(fixture)
	defer server.Close()
	path, cfg := authConfig(t, server.URL, "old-token")
	client, err := New(cfg, path)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if _, err := client.Login(context.Background(), domain.LoginRequest{
		Address: server.URL,
		Method:  domain.AuthToken,
		Token:   "invalid-token",
	}); err == nil || !domain.IsAuthenticationError(err) {
		t.Fatalf("invalid token Login() error = %v", err)
	}
	unchanged, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load(unchanged config) error = %v", err)
	}
	if unchanged.Vault.Token != "old-token" {
		t.Fatal("invalid token changed the config")
	}

	session, err := client.Login(context.Background(), domain.LoginRequest{
		Address: server.URL,
		Method:  domain.AuthToken,
		Token:   "new-token",
	})
	if err != nil {
		t.Fatalf("Login(token) error = %v", err)
	}
	if session.DisplayName != "new-user" {
		t.Fatalf("token session = %#v", session)
	}
	saved, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load(saved config) error = %v", err)
	}
	if saved.Vault.Token != "new-token" {
		t.Fatal("valid token was not saved")
	}
}

func authConfig(t *testing.T, address, token string) (string, config.Config) {
	t.Helper()
	path := filepath.Join(t.TempDir(), config.DefaultPath)
	cfg := config.Config{
		Vault: config.Vault{Address: address, Token: token, RequestTimeout: "2s"},
		UI:    config.UI{Color: "never", Language: "en"},
	}
	if err := config.SaveNew(path, cfg); err != nil {
		t.Fatalf("SaveNew() error = %v", err)
	}
	return path, cfg
}
