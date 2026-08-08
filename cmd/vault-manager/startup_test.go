package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vault-manager/internal/config"
)

func TestLoadStartupConfigRequiresTerminalWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.DefaultPath)
	if _, err := loadStartupConfig(path); err == nil || !strings.Contains(err.Error(), "requires a terminal") {
		t.Fatalf("loadStartupConfig() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("missing config was created without a terminal: %v", err)
	}
}

func TestLoadStartupConfigUsesCompleteFileWithoutTerminal(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.DefaultPath)
	want := config.Config{
		Vault: config.Vault{Address: "http://vault.wsoft:8400", Token: "test-token", RequestTimeout: "10s"},
		UI:    config.UI{Color: "auto", Language: "ru"},
	}
	if err := config.SaveNew(path, want); err != nil {
		t.Fatalf("SaveNew() error = %v", err)
	}
	got, err := loadStartupConfig(path)
	if err != nil {
		t.Fatalf("loadStartupConfig() error = %v", err)
	}
	if got.Path != path || got.Config.Vault.Address != want.Vault.Address || got.Config.Vault.Token != want.Vault.Token || got.Config.UI.Language != want.UI.Language {
		t.Fatalf("loaded config = %#v", got)
	}
}
