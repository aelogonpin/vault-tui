package vault

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/vault/api"
)

// tokenFile mirrors the path the official `vault` CLI uses, so a token
// obtained through this app can be reused by the CLI and vice versa.
func tokenFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".vault-token"), nil
}

func readCachedToken() string {
	path, err := tokenFile()
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func writeCachedToken(token string) error {
	path, err := tokenFile()
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(token), 0o600)
}

// NewClient builds a Vault API client from cfg, layered on top of whatever
// VAULT_* environment variables are already set (TLS settings, VAULT_ADDR
// as a fallback, etc).
func NewClient(cfg Config) (*api.Client, error) {
	apiCfg := api.DefaultConfig()
	if err := apiCfg.ReadEnvironment(); err != nil {
		return nil, fmt.Errorf("reading Vault env config: %w", err)
	}

	client, err := api.NewClient(apiCfg)
	if err != nil {
		return nil, fmt.Errorf("creating Vault client: %w", err)
	}

	if cfg.Addr != "" {
		if err := client.SetAddress(cfg.Addr); err != nil {
			return nil, fmt.Errorf("setting Vault address: %w", err)
		}
	}
	if client.Address() == "" {
		return nil, fmt.Errorf("no Vault address configured (set VAULT_ADDR or pass --addr)")
	}

	if cfg.Namespace != "" {
		client.SetNamespace(cfg.Namespace)
	}

	return client, nil
}

// EnsureAuthenticated makes sure client carries a valid token, reusing a
// cached one from ~/.vault-token when possible and falling back to an
// interactive OIDC browser login otherwise. On a fresh OIDC login the
// resulting token is cached for next time.
func EnsureAuthenticated(ctx context.Context, client *api.Client, cfg Config) error {
	if token := readCachedToken(); token != "" {
		client.SetToken(token)
		if _, err := client.Auth().Token().LookupSelfWithContext(ctx); err == nil {
			return nil
		}
		// Cached token is invalid/expired; fall through to a fresh login.
	}

	token, err := OIDCLogin(ctx, client, cfg)
	if err != nil {
		return fmt.Errorf("OIDC login failed: %w", err)
	}

	client.SetToken(token)
	if err := writeCachedToken(token); err != nil {
		// Non-fatal: we're authenticated for this session even if caching failed.
		return nil
	}
	return nil
}
