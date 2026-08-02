package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"vault-manager/internal/config"
	"vault-manager/internal/ui"
)

func loadStartupConfig(path string) (config.Config, error) {
	source, err := config.LoadForStartup(path)
	if err != nil {
		return config.Config{}, err
	}
	cfg := source.Config
	needsInput := !source.Exists || strings.TrimSpace(cfg.Vault.Address) == "" || strings.TrimSpace(cfg.Vault.Token) == ""
	if !needsInput {
		return cfg, cfg.Validate()
	}
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		return config.Config{}, errors.New("interactive configuration requires a terminal; provide vault.address and vault.token in the config file")
	}

	program := tea.NewProgram(ui.NewSetup(cfg, !source.Exists))
	result, err := program.Run()
	if err != nil {
		return config.Config{}, fmt.Errorf("configuration dialog: %w", err)
	}
	setup, ok := result.(*ui.SetupModel)
	if !ok {
		return config.Config{}, errors.New("configuration dialog returned an unexpected result")
	}
	if setup.Cancelled() {
		return config.Config{}, errors.New("configuration dialog cancelled")
	}
	cfg = setup.Config()
	if err := cfg.Validate(); err != nil {
		return config.Config{}, err
	}
	if !source.Exists {
		if err := config.SaveNew(source.Path, cfg); err != nil {
			return config.Config{}, err
		}
	}
	return cfg, nil
}
