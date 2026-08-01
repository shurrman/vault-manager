package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"vault-manager/internal/config"
	"vault-manager/internal/domain"
	"vault-manager/internal/i18n"
	"vault-manager/internal/ui"
	"vault-manager/internal/vaultapi"
)

var version = "0.2"

func main() {
	os.Exit(run())
}

func run() int {
	configPath := flag.String("config", "", "path to vault-manager TOML configuration (default: ~/"+config.DefaultPath+")")
	check := flag.Bool("check", false, "perform a read-only API check and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("vault-manager", version)
		return 0
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		return 2
	}
	client, err := vaultapi.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Vault client error:", err)
		return 2
	}
	if *check {
		return checkVault(client, cfg.UI.Language)
	}

	program := tea.NewProgram(ui.New(client, cfg.UI.Color, cfg.UI.Language))
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "vault-manager error:", err)
		return 1
	}
	return 0
}

func checkVault(client domain.Backend, languages ...string) int {
	language := "en"
	if len(languages) > 0 {
		language = languages[0]
	}
	text := i18n.New(language)
	ctx := context.Background()
	failures := 0
	for _, section := range domain.Sections {
		items, err := client.List(ctx, section)
		if err != nil {
			fmt.Printf("%-12s error: %v\n", text.Text(section.Label()), err)
			failures++
			continue
		}
		if len(items) > 0 {
			if _, err := client.Preview(ctx, section, items[0]); err != nil {
				fmt.Printf("%-12s list ok (%d items), preview error: %v\n", text.Text(section.Label()), len(items), err)
				failures++
				continue
			}
		}
		fmt.Printf("%-12s ok (%d items)\n", text.Text(section.Label()), len(items))
	}
	if failures > 0 {
		return 1
	}
	return 0
}
