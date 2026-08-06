package vault

// Config holds the connection and auth settings needed to reach Vault.
// Values are normally sourced from flags with environment variables as
// fallback (see cmd flag wiring in main.go).
type Config struct {
	Addr      string // VAULT_ADDR
	Namespace string // VAULT_NAMESPACE, optional (Vault Enterprise)
	OIDCMount string // auth mount path for the OIDC method, e.g. "oidc"
	OIDCRole  string // role to request during OIDC login, optional
	OIDCPort  int    // local callback port; 0 = default (8250, same as `vault login -method=oidc`)
}
