package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	path := writeConfig(t, 0o600, `[vault]
address = "http://127.0.0.1:8200"
token = "test-token"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Vault.RequestTimeout != "10s" || cfg.UI.Color != "auto" || cfg.UI.Language != "en" {
		t.Fatalf("defaults not applied: %#v", cfg)
	}
}

func TestLoadOptionalDeploymentNotice(t *testing.T) {
	path := writeConfig(t, 0o600, `[vault]
address = "http://127.0.0.1:8200"
token = "test-token"

[deployment]
membership_notice = "Membership is managed externally."
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Deployment.MembershipNotice != "Membership is managed externally." {
		t.Fatalf("membership notice = %q", cfg.Deployment.MembershipNotice)
	}
}

func TestLoadRejectsOpenPermissions(t *testing.T) {
	path := writeConfig(t, 0o644, `[vault]
address = "http://127.0.0.1:8200"
token = "secret-that-must-not-appear"
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "group or others") {
		t.Fatalf("Load() error = %v", err)
	}
	if strings.Contains(err.Error(), "secret-that-must-not-appear") {
		t.Fatal("Load() exposed the token")
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := writeConfig(t, 0o600, `[vault]
address = "http://127.0.0.1:8200"
token = "test-token"
unexpected = true
`)

	if _, err := Load(path); err == nil {
		t.Fatal("Load() accepted an unknown field")
	}
}

func TestValidateRejectsInvalidValuesWithoutToken(t *testing.T) {
	cfg := Config{
		Vault: Vault{Address: "not-a-url", Token: "secret-that-must-not-appear", RequestTimeout: "10s"},
		UI:    UI{Color: "auto"},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() accepted an invalid address")
	}
	if strings.Contains(err.Error(), cfg.Vault.Token) {
		t.Fatal("Validate() exposed the token")
	}
}

func writeConfig(t *testing.T, mode os.FileMode, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadUILanguage(t *testing.T) {
	path := writeConfig(t, 0o600, `[vault]
address = "http://vault.wsoft:8400"
token = "test-token"

[ui]
language = "ru"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.UI.Language != "ru" {
		t.Fatalf("ui.language = %q", cfg.UI.Language)
	}
}

func TestLoadRejectsUnsupportedUILanguage(t *testing.T) {
	path := writeConfig(t, 0o600, `[vault]
address = "http://vault.wsoft:8400"
token = "test-token"

[ui]
language = "de"
`)
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "ui.language") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadDefaultPathUsesHomeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, DefaultPath)
	if err := os.WriteFile(path, []byte(`[vault]
address = "http://vault.wsoft:8400"
token = "test-token"
`), 0o600); err != nil {
		t.Fatalf("write default config: %v", err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(default) error = %v", err)
	}
	if cfg.Vault.Address != "http://vault.wsoft:8400" {
		t.Fatalf("vault.address = %q", cfg.Vault.Address)
	}
}
