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

func TestLoadForStartupAllowsMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	source, err := LoadForStartup(path)
	if err != nil {
		t.Fatalf("LoadForStartup() error = %v", err)
	}
	if source.Exists || source.Path != path {
		t.Fatalf("source = %#v", source)
	}
	if source.Config.Vault.RequestTimeout != "10s" || source.Config.UI.Color != "auto" || source.Config.UI.Language != "en" {
		t.Fatalf("defaults not applied: %#v", source.Config)
	}
}

func TestLoadForStartupAllowsMissingRequiredValues(t *testing.T) {
	path := writeConfig(t, 0o600, `[ui]
language = "ru"
`)
	source, err := LoadForStartup(path)
	if err != nil {
		t.Fatalf("LoadForStartup() error = %v", err)
	}
	if !source.Exists || source.Config.UI.Language != "ru" {
		t.Fatalf("source = %#v", source)
	}
	if err := source.Config.Validate(); err == nil || !strings.Contains(err.Error(), "vault.address") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestLoadForStartupRejectsInvalidPresentAddress(t *testing.T) {
	path := writeConfig(t, 0o600, `[vault]
address = "not-a-url"
`)
	if _, err := LoadForStartup(path); err == nil || !strings.Contains(err.Error(), "absolute http") {
		t.Fatalf("LoadForStartup() error = %v", err)
	}
}

func TestLoadForStartupRejectsOpenPermissions(t *testing.T) {
	path := writeConfig(t, 0o644, `[vault]
address = "http://vault.wsoft:8400"
`)
	if _, err := LoadForStartup(path); err == nil || !strings.Contains(err.Error(), "group or others") {
		t.Fatalf("LoadForStartup() error = %v", err)
	}
}

func TestLoadForStartupRejectsUnknownFields(t *testing.T) {
	path := writeConfig(t, 0o600, `[vault]
unexpected = true
`)
	if _, err := LoadForStartup(path); err == nil {
		t.Fatalf("LoadForStartup() error = %v", err)
	}
}

func TestSaveNewCreatesOwnerOnlyConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	cfg := Config{
		Vault: Vault{Address: "http://vault.wsoft:8400", Token: "test-token", RequestTimeout: "10s"},
		UI:    UI{Color: "auto", Language: "ru"},
	}
	if err := SaveNew(path, cfg); err != nil {
		t.Fatalf("SaveNew() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat saved config: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("saved mode = %o", info.Mode().Perm())
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load(saved) error = %v", err)
	}
	if loaded.Vault.Address != cfg.Vault.Address || loaded.Vault.Token != cfg.Vault.Token || loaded.UI.Language != "ru" {
		t.Fatalf("saved config = %#v", loaded)
	}
}

func TestSaveNewDoesNotOverwriteExistingFile(t *testing.T) {
	path := writeConfig(t, 0o600, `[vault]
address = "http://original.example:8200"
token = "original-token"
`)
	cfg := Config{
		Vault: Vault{Address: "http://replacement.example:8200", Token: "replacement-token", RequestTimeout: "10s"},
		UI:    UI{Color: "auto", Language: "en"},
	}
	if err := SaveNew(path, cfg); err == nil {
		t.Fatal("SaveNew() overwrote an existing file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read original config: %v", err)
	}
	if !strings.Contains(string(data), "original-token") || strings.Contains(string(data), "replacement-token") {
		t.Fatal("existing config content changed")
	}
}

func TestSaveCredentialsUpdatesOnlyVaultCredentials(t *testing.T) {
	path := writeConfig(t, 0o600, "[vault]\naddress = \"http://old.example:8200\"\ntoken = \"old-token\"\nrequest_timeout = \"17s\"\n\n[ui]\ncolor = \"never\"\nlanguage = \"ru\"\n\n[deployment]\nmembership_notice = \"managed elsewhere\"\n")
	if err := SaveCredentials(path, "http://new.example:8400", "new-token"); err != nil {
		t.Fatalf("SaveCredentials() error = %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(updated) error = %v", err)
	}
	if cfg.Vault.Address != "http://new.example:8400" || cfg.Vault.Token != "new-token" {
		t.Fatalf("updated credentials = address %q token changed=%v", cfg.Vault.Address, cfg.Vault.Token == "new-token")
	}
	if cfg.Vault.RequestTimeout != "17s" || cfg.UI.Color != "never" || cfg.UI.Language != "ru" || cfg.Deployment.MembershipNotice != "managed elsewhere" {
		t.Fatalf("unrelated configuration changed: %#v", cfg)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat updated config: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("updated config mode = %o", info.Mode().Perm())
	}
}

func TestSaveCredentialsRejectsInvalidValuesWithoutChangingFile(t *testing.T) {
	path := writeConfig(t, 0o600, "[vault]\naddress = \"http://old.example:8200\"\ntoken = \"old-token\"\n")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read before update: %v", err)
	}
	if err := SaveCredentials(path, "not-a-url", "new-token"); err == nil {
		t.Fatal("SaveCredentials() accepted an invalid address")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after update: %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("invalid credentials changed the config")
	}
}
