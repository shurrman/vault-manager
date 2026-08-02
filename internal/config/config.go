package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
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
	Color    string `toml:"color"`
	Language string `toml:"language"`
}

type Deployment struct {
	MembershipNotice string `toml:"membership_notice"`
}

type StartupSource struct {
	Config Config
	Path   string
	Exists bool
}

func Load(path string) (Config, error) {
	path, err := resolvePath(path)
	if err != nil {
		return Config{}, err
	}
	cfg, _, err := loadFile(path, false)
	if err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func LoadForStartup(path string) (StartupSource, error) {
	path, err := resolvePath(path)
	if err != nil {
		return StartupSource{}, err
	}
	cfg, exists, err := loadFile(path, true)
	if err != nil {
		return StartupSource{}, err
	}
	if err := cfg.ValidatePartial(); err != nil {
		return StartupSource{}, err
	}
	return StartupSource{Config: cfg, Path: path, Exists: exists}, nil
}

func SaveNew(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create config: %w", err)
	}
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(path)
	}
	if _, err := file.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write config: %w", err)
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync config: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("close config: %w", err)
	}
	return nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Vault.Address) == "" {
		return errors.New("vault.address is required")
	}
	if err := validateAddress(c.Vault.Address); err != nil {
		return err
	}
	if strings.TrimSpace(c.Vault.Token) == "" {
		return errors.New("vault.token is required")
	}
	return c.validateSettings()
}

func (c Config) ValidatePartial() error {
	if strings.TrimSpace(c.Vault.Address) != "" {
		if err := validateAddress(c.Vault.Address); err != nil {
			return err
		}
	}
	return c.validateSettings()
}

func (c Config) validateSettings() error {
	if _, err := c.Timeout(); err != nil {
		return err
	}
	switch c.UI.Color {
	case "auto", "always", "never":
	default:
		return errors.New("ui.color must be auto, always, or never")
	}
	switch c.UI.Language {
	case "en", "ru", "es", "zh":
		return nil
	default:
		return errors.New("ui.language must be en, ru, es, or zh")
	}
}

func resolvePath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, DefaultPath), nil
}

func loadFile(path string, allowMissing bool) (Config, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if allowMissing && errors.Is(err, os.ErrNotExist) {
			var cfg Config
			applyDefaults(&cfg)
			return cfg, false, nil
		}
		return Config{}, false, fmt.Errorf("stat config: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return Config{}, false, fmt.Errorf("config %q must not be accessible by group or others", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, false, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	decoder := toml.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, false, fmt.Errorf("decode config: %w", err)
	}
	applyDefaults(&cfg)
	return cfg, true, nil
}

func validateAddress(address string) error {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("vault.address must be an absolute http or https URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("vault.address must not contain credentials, query, or fragment")
	}
	return nil
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
	if c.UI.Language == "" {
		c.UI.Language = "en"
	}
}
