package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"vault-tui/internal/tui"
	"vault-tui/internal/vault"
)

func main() {
	var cfg vault.Config
	flag.StringVar(&cfg.Addr, "addr", os.Getenv("VAULT_ADDR"), "Vault server address (env VAULT_ADDR)")
	flag.StringVar(&cfg.Namespace, "namespace", os.Getenv("VAULT_NAMESPACE"), "Vault namespace, Enterprise only (env VAULT_NAMESPACE)")
	flag.StringVar(&cfg.AuthMethod, "auth-method", envOr("VAULT_AUTH_METHOD", "oidc"), "auth method: oidc, token, userpass, or ldap (env VAULT_AUTH_METHOD)")
	flag.StringVar(&cfg.OIDCMount, "oidc-mount", envOr("VAULT_OIDC_MOUNT", "oidc"), "auth mount path for the OIDC method (env VAULT_OIDC_MOUNT)")
	flag.StringVar(&cfg.OIDCRole, "oidc-role", os.Getenv("VAULT_OIDC_ROLE"), "OIDC role to request at login (env VAULT_OIDC_ROLE)")
	flag.IntVar(&cfg.OIDCPort, "oidc-port", envIntOr("VAULT_OIDC_PORT", 0), "local OIDC callback port; 0 = try 8250-8259 like `vault login -method=oidc` (env VAULT_OIDC_PORT)")
	flag.StringVar(&cfg.Token, "token", os.Getenv("VAULT_TOKEN"), "Vault token to use; required when --auth-method=token (env VAULT_TOKEN)")
	flag.StringVar(&cfg.Username, "username", os.Getenv("VAULT_USERNAME"), "username for userpass/ldap login; prompted if empty (env VAULT_USERNAME)")
	flag.StringVar(&cfg.Password, "password", os.Getenv("VAULT_PASSWORD"), "password for userpass/ldap login; prompted (hidden) if empty — prefer the prompt over this flag on shared machines (env VAULT_PASSWORD)")
	flag.StringVar(&cfg.UserpassMount, "userpass-mount", envOr("VAULT_USERPASS_MOUNT", "userpass"), "auth mount path for the userpass method (env VAULT_USERPASS_MOUNT)")
	flag.StringVar(&cfg.LDAPMount, "ldap-mount", envOr("VAULT_LDAP_MOUNT", "ldap"), "auth mount path for the LDAP method (env VAULT_LDAP_MOUNT)")
	debugLog := flag.String("debug-log", os.Getenv("VAULT_TUI_DEBUG_LOG"), "write debug logs to this file (env VAULT_TUI_DEBUG_LOG); the TUI occupies the terminal so this is the only way to see log output while it runs")
	flag.Parse()

	if err := run(cfg, *debugLog); err != nil {
		fmt.Fprintln(os.Stderr, "vault-tui:", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func run(cfg vault.Config, debugLog string) error {
	if debugLog != "" {
		f, err := tea.LogToFile(debugLog, "vault-tui")
		if err != nil {
			return fmt.Errorf("opening debug log: %w", err)
		}
		defer f.Close()
		log.Printf("--- vault-tui starting, addr=%s ---", cfg.Addr)
	} else {
		// The TUI occupies the whole terminal; without an explicit
		// --debug-log target, any stray log output would corrupt the
		// display. Discard it instead of falling back to the log
		// package's default (stderr).
		log.SetOutput(io.Discard)
	}

	client, err := vault.NewClient(cfg)
	if err != nil {
		return err
	}

	ctx := context.Background()
	fmt.Println("Authenticating against", client.Address())
	if err := vault.EnsureAuthenticated(ctx, client, cfg); err != nil {
		return err
	}
	fmt.Println("Authenticated. Launching...")

	p := tea.NewProgram(tui.New(client, cfg), tea.WithAltScreen())
	_, err = p.Run()
	return err
}
