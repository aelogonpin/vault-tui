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
// cached one from ~/.vault-token when possible and falling back to
// cfg.AuthMethod's login flow otherwise (OIDC browser login by default; see
// login). On a fresh login the resulting token is cached for next time.
func EnsureAuthenticated(ctx context.Context, client *api.Client, cfg Config) error {
	// An explicit --token bypasses the cache: the caller handed us a token
	// to use right now, not a hint to look one up.
	if cfg.AuthMethod == "token" && cfg.Token != "" {
		client.SetToken(cfg.Token)
		if _, err := client.Auth().Token().LookupSelfWithContext(ctx); err != nil {
			return fmt.Errorf("token login failed: %w", err)
		}
		_ = writeCachedToken(cfg.Token) // non-fatal if caching fails
		return nil
	}

	if token := readCachedToken(); token != "" {
		client.SetToken(token)
		if _, err := client.Auth().Token().LookupSelfWithContext(ctx); err == nil {
			return nil
		}
		// Cached token is invalid/expired; fall through to a fresh login.
	}

	token, err := login(ctx, client, cfg)
	if err != nil {
		return err
	}

	client.SetToken(token)
	if err := writeCachedToken(token); err != nil {
		// Non-fatal: we're authenticated for this session even if caching failed.
		return nil
	}
	return nil
}

// login runs the interactive login flow selected by cfg.AuthMethod,
// defaulting to OIDC for backwards compatibility with existing configs that
// don't set it.
func login(ctx context.Context, client *api.Client, cfg Config) (string, error) {
	switch cfg.AuthMethod {
	case "", "oidc":
		token, err := OIDCLogin(ctx, client, cfg)
		if err != nil {
			return "", fmt.Errorf("OIDC login failed: %w", err)
		}
		return token, nil

	case "token":
		return "", fmt.Errorf("--auth-method=token requires --token (or VAULT_TOKEN) to be set")

	case "userpass":
		mount := cfg.UserpassMount
		if mount == "" {
			mount = "userpass"
		}
		return passwordLogin(ctx, client, mount, cfg, "userpass")

	case "ldap":
		mount := cfg.LDAPMount
		if mount == "" {
			mount = "ldap"
		}
		return passwordLogin(ctx, client, mount, cfg, "LDAP")

	default:
		return "", fmt.Errorf("unknown --auth-method %q (want oidc, token, userpass, or ldap)", cfg.AuthMethod)
	}
}

// passwordLogin drives userpass/LDAP login: it fills in username/password
// from cfg if provided, prompting interactively for whichever is missing,
// then exchanges them for a token against mount.
func passwordLogin(ctx context.Context, client *api.Client, mount string, cfg Config, label string) (string, error) {
	username := cfg.Username
	if username == "" {
		u, err := promptString(fmt.Sprintf("%s username: ", label))
		if err != nil {
			return "", fmt.Errorf("reading username: %w", err)
		}
		username = u
	}

	password := cfg.Password
	if password == "" {
		p, err := promptPassword(fmt.Sprintf("%s password: ", label))
		if err != nil {
			return "", fmt.Errorf("reading password: %w", err)
		}
		password = p
	}

	token, err := PasswordLogin(ctx, client, mount, username, password)
	if err != nil {
		return "", fmt.Errorf("%s login failed: %w", label, err)
	}
	return token, nil
}
