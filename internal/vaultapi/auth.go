package vaultapi

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/vault/api"

	"vault-manager/internal/config"
	"vault-manager/internal/domain"
)

func (c *Client) Address() string {
	c.sessionMu.RLock()
	defer c.sessionMu.RUnlock()
	return c.cfg.Vault.Address
}

func (c *Client) Session(ctx context.Context) (domain.Session, error) {
	c.sessionMu.RLock()
	defer c.sessionMu.RUnlock()
	return sessionInfo(ctx, c.api, c.cfg.Vault.Address)
}

func (c *Client) Login(ctx context.Context, request domain.LoginRequest) (domain.Session, error) {
	address := strings.TrimSpace(request.Address)
	if address == "" {
		return domain.Session{}, errors.New("vault.address is required")
	}
	c.sessionMu.RLock()
	cfg := c.cfg
	configPath := c.configPath
	c.sessionMu.RUnlock()
	if configPath == "" {
		return domain.Session{}, errors.New("configuration path is unavailable")
	}
	cfg.Vault.Address = address
	cfg.Vault.Token = ""
	if err := cfg.ValidatePartial(); err != nil {
		return domain.Session{}, err
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		return domain.Session{}, err
	}

	var token string
	switch request.Method {
	case domain.AuthToken:
		token = strings.TrimSpace(request.Token)
		if token == "" {
			return domain.Session{}, errors.New("token is required")
		}
	case domain.AuthUserpass:
		username := strings.TrimSpace(request.Username)
		if username == "" {
			return domain.Session{}, errors.New("username is required")
		}
		if request.Password == "" {
			return domain.Session{}, errors.New("password is required")
		}
		secret, loginErr := client.Logical().WriteWithContext(ctx, "auth/userpass/login/"+url.PathEscape(username), map[string]interface{}{
			"password": request.Password,
		})
		if loginErr != nil {
			return domain.Session{}, authenticationPublicError("login", loginErr)
		}
		if secret == nil || secret.Auth == nil || strings.TrimSpace(secret.Auth.ClientToken) == "" {
			return domain.Session{}, errors.New("authentication response did not contain a token")
		}
		token = secret.Auth.ClientToken
	default:
		return domain.Session{}, fmt.Errorf("unsupported authentication method %q", request.Method)
	}

	client.SetToken(token)
	session, err := sessionInfo(ctx, client, address)
	if err != nil {
		return domain.Session{}, err
	}
	if request.Method == domain.AuthUserpass {
		session.DisplayName = strings.TrimSpace(request.Username)
	}

	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	if err := config.SaveCredentials(configPath, address, token); err != nil {
		return domain.Session{}, fmt.Errorf("save credentials: %w", err)
	}
	c.api = client
	c.cfg.Vault.Address = address
	c.cfg.Vault.Token = token
	c.clearSessionCaches()
	return session, nil
}

func sessionInfo(ctx context.Context, client *api.Client, address string) (domain.Session, error) {
	secret, err := client.Logical().ReadWithContext(ctx, "auth/token/lookup-self")
	if err != nil {
		return domain.Session{}, authenticationPublicError("authenticate", err)
	}
	if secret == nil {
		return domain.Session{}, &domain.AuthenticationError{Err: errors.New("authenticate: Vault returned no token information")}
	}
	data := secretData(secret)
	displayName := stringValue(interfaceMap(data["meta"])["username"])
	if displayName == "" {
		displayName = stringValue(data["display_name"])
	}
	if displayName == "" {
		displayName = "token"
	}
	return domain.Session{Address: address, DisplayName: displayName}, nil
}

func authenticationPublicError(operation string, err error) error {
	var responseErr *api.ResponseError
	if errors.As(err, &responseErr) && (responseErr.StatusCode == 401 || responseErr.StatusCode == 403) {
		return &domain.AuthenticationError{Err: fmt.Errorf("%s: Vault returned HTTP %d", operation, responseErr.StatusCode)}
	}
	return publicError(operation, err)
}

func (c *Client) clearSessionCaches() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entityNames = make(map[string]string)
	c.groupNames = make(map[string]string)
	c.authByAccessor = make(map[string]authMount)
	c.userRefs = make(map[string]userRef)
}
