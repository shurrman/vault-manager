package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const DefaultPath = ".vault-manager"

type Config struct {
	Vault      Vault      `toml:"vault"`
	UI         UI         `toml:"ui"`
	Deployment Deployment `toml:"deployment"`
}

type Vault struct {
	Address        string `toml:"address"`
	Token          string `toml:"token"`
	RequestTimeout string `toml:"request_timeout"`
}

type UI struct {
	Color string `toml:"color"`
}

type Deployment struct {
	MembershipNotice string `toml:"membership_notice"`
}

func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultPath
	}

	info, err := os.Stat(path)
	if err != nil {
		return Config{}, fmt.Errorf("stat config: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return Config{}, fmt.Errorf("config %q must not be accessible by group or others", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	decoder := toml.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	applyDefaults(&cfg)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Vault.Address) == "" {
		return errors.New("vault.address is required")
	}
	parsed, err := url.Parse(c.Vault.Address)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("vault.address must be an absolute http or https URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("vault.address must not contain credentials, query, or fragment")
	}
	if strings.TrimSpace(c.Vault.Token) == "" {
		return errors.New("vault.token is required")
	}
	if _, err := c.Timeout(); err != nil {
		return err
	}
	switch c.UI.Color {
	case "auto", "always", "never":
		return nil
	default:
		return errors.New("ui.color must be auto, always, or never")
	}
}

func (c Config) Timeout() (time.Duration, error) {
	timeout, err := time.ParseDuration(c.Vault.RequestTimeout)
	if err != nil || timeout <= 0 {
		return 0, errors.New("vault.request_timeout must be a positive duration")
	}
	return timeout, nil
}

func applyDefaults(c *Config) {
	if c.Vault.RequestTimeout == "" {
		c.Vault.RequestTimeout = "10s"
	}
	if c.UI.Color == "" {
		c.UI.Color = "auto"
	}
}
