package vault

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/hashicorp/vault/api"
)

// loginTimeout bounds how long we wait for the user to complete the
// browser-based login before giving up.
const loginTimeout = 3 * time.Minute

const callbackPath = "/oidc/callback"

// defaultOIDCPort matches the port `vault login -method=oidc` itself binds
// to. Vault admins commonly whitelist exactly this localhost redirect URI
// (or a small range starting here) in the OIDC role's allowed_redirect_uris
// and in the identity provider's registered redirect URIs, precisely so
// CLI-style logins work without extra config. Using the same port/range
// here means this app is likely to work out of the box wherever `vault
// login -method=oidc` already does.
const defaultOIDCPort = 8250

// fallbackPortRange is how many consecutive ports past defaultOIDCPort we
// try before giving up, but only when the port wasn't explicitly pinned via
// cfg.OIDCPort/--oidc-port/VAULT_OIDC_PORT.
const fallbackPortRange = 10

// listenLocal opens the local callback listener. If cfg.OIDCPort is set
// (non-zero) it's used exactly, failing loudly if unavailable — the caller
// asked for that specific port because it's the one whitelisted on the
// Vault/IdP side. Otherwise it tries defaultOIDCPort and a small range of
// fallbacks, same as the official Vault CLI.
func listenLocal(cfg Config) (net.Listener, int, error) {
	if cfg.OIDCPort != 0 {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.OIDCPort))
		if err != nil {
			return nil, 0, fmt.Errorf("binding local callback port %d (pinned via --oidc-port/VAULT_OIDC_PORT): %w", cfg.OIDCPort, err)
		}
		return l, cfg.OIDCPort, nil
	}

	var lastErr error
	for port := defaultOIDCPort; port < defaultOIDCPort+fallbackPortRange; port++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			return l, port, nil
		}
		lastErr = err
	}
	return nil, 0, fmt.Errorf("no free local port in %d-%d for the OIDC callback: %w", defaultOIDCPort, defaultOIDCPort+fallbackPortRange-1, lastErr)
}

// OIDCLogin drives the same browser-based flow as `vault login
// -method=oidc`: it asks Vault for a provider auth URL bound to a local
// callback address, opens that URL in the user's browser, waits for the
// provider to redirect back, and forwards the result to Vault to mint a
// client token.
func OIDCLogin(ctx context.Context, client *api.Client, cfg Config) (string, error) {
	if cfg.OIDCMount == "" {
		cfg.OIDCMount = "oidc"
	}

	listener, port, err := listenLocal(cfg)
	if err != nil {
		return "", err
	}
	defer listener.Close()

	redirectURI := fmt.Sprintf("http://localhost:%d%s", port, callbackPath)

	authURLSecret, err := client.Logical().WriteWithContext(ctx, fmt.Sprintf("auth/%s/oidc/auth_url", cfg.OIDCMount), map[string]interface{}{
		"role":         cfg.OIDCRole,
		"redirect_uri": redirectURI,
	})
	if err != nil {
		return "", fmt.Errorf("requesting OIDC auth URL: %w", err)
	}
	if authURLSecret == nil {
		return "", fmt.Errorf("Vault returned no data for OIDC auth URL (check --oidc-mount / --oidc-role)")
	}
	authURL, _ := authURLSecret.Data["auth_url"].(string)
	if authURL == "" {
		return "", fmt.Errorf("Vault response missing auth_url")
	}

	type result struct {
		token string
		err   error
	}
	resultCh := make(chan result, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		secret, err := client.Logical().ReadWithDataWithContext(r.Context(), fmt.Sprintf("auth/%s/oidc/callback", cfg.OIDCMount), r.URL.Query())
		if err != nil || secret == nil || secret.Auth == nil {
			writeCallbackPage(w, false)
			if err == nil {
				err = fmt.Errorf("Vault OIDC callback returned no auth data")
			}
			resultCh <- result{err: err}
			return
		}
		writeCallbackPage(w, true)
		resultCh <- result{token: secret.Auth.ClientToken}
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)
	defer srv.Close()

	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Open this URL in your browser to log in:\n\n  %s\n\n", authURL)
	} else {
		fmt.Printf("Opening your browser to complete the OIDC login...\nIf it didn't open, visit:\n\n  %s\n\n", authURL)
	}

	loginCtx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()

	select {
	case res := <-resultCh:
		if res.err != nil {
			return "", res.err
		}
		return res.token, nil
	case <-loginCtx.Done():
		return "", fmt.Errorf("timed out waiting for OIDC login to complete")
	}
}

func writeCallbackPage(w http.ResponseWriter, ok bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if ok {
		fmt.Fprint(w, "<html><body><h3>Login successful</h3>You can close this tab and return to the terminal.</body></html>")
		return
	}
	w.WriteHeader(http.StatusBadGateway)
	fmt.Fprint(w, "<html><body><h3>Login failed</h3>Return to the terminal for details.</body></html>")
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
