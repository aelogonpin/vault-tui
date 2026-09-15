package vault

import (
	"context"
	"fmt"

	"github.com/hashicorp/vault/api"
)

// PasswordLogin authenticates against Vault's userpass or LDAP auth
// methods, which share the same login API shape: a write to
// auth/<mount>/login/<username> with a "password" field. The caller picks
// the mount ("userpass" or "ldap", or a custom mount path).
func PasswordLogin(ctx context.Context, client *api.Client, mount, username, password string) (string, error) {
	secret, err := client.Logical().WriteWithContext(ctx, fmt.Sprintf("auth/%s/login/%s", mount, username), map[string]interface{}{
		"password": password,
	})
	if err != nil {
		return "", fmt.Errorf("logging in: %w", err)
	}
	if secret == nil || secret.Auth == nil {
		return "", fmt.Errorf("Vault returned no auth data")
	}
	return secret.Auth.ClientToken, nil
}
