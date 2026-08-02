package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vault-manager/internal/config"
)

func TestSetupCollectsLanguageAddressAndMaskedToken(t *testing.T) {
	cfg := config.Config{
		Vault: config.Vault{RequestTimeout: "10s"},
		UI:    config.UI{Color: "auto", Language: "en"},
	}
	model := NewSetup(cfg, true)
	if view := model.View().Content; !strings.Contains(view, "English") || !strings.Contains(view, "Русский") {
		t.Fatalf("language dropdown = %q", view)
	}

	model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if model.config.UI.Language != "ru" || model.step != setupAddress {
		t.Fatalf("language selection = %#v step=%v", model.config.UI, model.step)
	}
	if !strings.Contains(model.View().Content, "Введите URL Vault") {
		t.Fatalf("URL prompt was not translated: %q", model.View().Content)
	}

	model.input.SetValue("http://vault.wsoft:8400")
	model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if model.step != setupToken {
		t.Fatalf("step after address = %v", model.step)
	}
	model.input.SetValue("root-token")
	if strings.Contains(model.View().Content, "root-token") {
		t.Fatal("token is visible in setup view")
	}
	model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if model.step != setupDone || model.config.Vault.Token != "root-token" {
		t.Fatalf("completed setup = %#v step=%v", model.config, model.step)
	}
}

func TestSetupRejectsInvalidAddress(t *testing.T) {
	cfg := config.Config{
		Vault: config.Vault{RequestTimeout: "10s"},
		UI:    config.UI{Color: "auto", Language: "en"},
	}
	model := NewSetup(cfg, false)
	model.input.SetValue("not-a-url")
	model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if model.step != setupAddress || !strings.Contains(model.err, "absolute http") {
		t.Fatalf("invalid address state = step %v error %q", model.step, model.err)
	}
}

func TestSetupTranslatesValidationError(t *testing.T) {
	cfg := config.Config{
		Vault: config.Vault{RequestTimeout: "10s"},
		UI:    config.UI{Color: "auto", Language: "ru"},
	}
	model := NewSetup(cfg, false)
	model.input.SetValue("not-a-url")
	model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(model.err, "абсолютным URL") {
		t.Fatalf("translated validation error = %q", model.err)
	}
}

func TestSetupAsksOnlyForMissingToken(t *testing.T) {
	cfg := config.Config{
		Vault: config.Vault{Address: "http://vault.wsoft:8400", RequestTimeout: "10s"},
		UI:    config.UI{Color: "auto", Language: "es"},
	}
	model := NewSetup(cfg, false)
	if model.step != setupToken || !strings.Contains(model.View().Content, "token de Vault") {
		t.Fatalf("token-only setup = step %v view %q", model.step, model.View().Content)
	}
}

func TestSetupCanBeCancelled(t *testing.T) {
	cfg := config.Config{
		Vault: config.Vault{RequestTimeout: "10s"},
		UI:    config.UI{Color: "auto", Language: "en"},
	}
	model := NewSetup(cfg, true)
	model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !model.Cancelled() {
		t.Fatal("setup was not cancelled")
	}
}
