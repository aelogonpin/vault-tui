package vault

// Config holds the connection and auth settings needed to reach Vault.
// Values are normally sourced from flags with environment variables as
// fallback (see cmd flag wiring in main.go).
type Config struct {
	Addr      string // VAULT_ADDR
	Namespace string // VAULT_NAMESPACE, optional (Vault Enterprise)

	AuthMethod string // VAULT_AUTH_METHOD: "oidc" (default), "token", "userpass", or "ldap"

	OIDCMount string // auth mount path for the OIDC method, e.g. "oidc"
	OIDCRole  string // role to request during OIDC login, optional
	OIDCPort  int    // local callback port; 0 = default (8250, same as `vault login -method=oidc`)

	Token string // VAULT_TOKEN; used directly when AuthMethod == "token"

	Username      string // VAULT_USERNAME; used when AuthMethod == "userpass" or "ldap", prompted if empty
	Password      string // VAULT_PASSWORD; used when AuthMethod == "userpass" or "ldap", prompted (hidden) if empty
	UserpassMount string // auth mount path for the userpass method, e.g. "userpass"
	LDAPMount     string // auth mount path for the LDAP method, e.g. "ldap"
}
