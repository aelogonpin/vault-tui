# vault-tui

Interactive terminal UI (Bubble Tea) for browsing and managing secrets in a
HashiCorp Vault KV v2 engine: browse mounts/folders, view a secret's
key/value pairs and version history, create secrets, save edits as new
versions, rename secrets/folders, and delete (soft-delete or permanently
destroy).

## Requirements

- Go 1.26+ (installed via Homebrew: `brew install go`)
- Network access to your Vault server
- One of the auth methods below enabled on that Vault server

## Build

```sh
go build -o vault-tui .
```

## Install a prebuilt release

Every tag pushed to this repo (`vX.Y.Z`) triggers a GitHub Actions workflow
that cross-compiles binaries for Linux, macOS and Windows (amd64 + arm64,
Windows amd64 only) and attaches them, plus a `checksums.txt`, to a GitHub
Release. Grab the archive for your platform from the
[Releases](../../releases) page, extract it, and run the `vault-tui`
(or `vault-tui.exe`) binary directly — no Go toolchain needed.

To cut a new release yourself:

```sh
git tag v0.1.0
git push origin v0.1.0
```

## Run

```sh
./vault-tui --addr https://vault.example.com
```

Or set environment variables instead of flags:

| Flag           | Env var            | Default   | Notes                                   |
|----------------|---------------------|-----------|------------------------------------------|
| `--addr`       | `VAULT_ADDR`        | —         | required                                 |
| `--namespace`  | `VAULT_NAMESPACE`   | —         | Vault Enterprise only                    |
| `--auth-method`| `VAULT_AUTH_METHOD` | `oidc`    | `oidc`, `token`, `userpass`, or `ldap`   |
| `--debug-log`  | `VAULT_TUI_DEBUG_LOG` | —       | write debug logs (key events, filter results, operation errors) to this file — the TUI occupies the terminal, so this is the only way to see log output while it's running |

### `--auth-method=oidc` (default)

| Flag           | Env var            | Default   | Notes                                   |
|----------------|---------------------|-----------|------------------------------------------|
| `--oidc-mount` | `VAULT_OIDC_MOUNT`  | `oidc`    | mount path of your OIDC auth method — check with your Vault admin if unsure |
| `--oidc-role`  | `VAULT_OIDC_ROLE`   | —         | leave empty to use the mount's default role |
| `--oidc-port`  | `VAULT_OIDC_PORT`   | `8250`    | local OIDC callback port; falls back to 8251-8259 if busy |

Opens your browser to your OIDC provider, waits for the redirect on a local
`localhost` callback, and exchanges it for a Vault token.

### `--auth-method=token`

| Flag       | Env var       | Default | Notes                              |
|------------|---------------|---------|--------------------------------------|
| `--token`  | `VAULT_TOKEN` | —       | required; an existing Vault token to use as-is |

Uses the token directly (after validating it with a self-lookup) — no
interactive login step. Useful if you already have a token from another
tool or auth method.

### `--auth-method=userpass` / `--auth-method=ldap`

| Flag               | Env var                | Default    | Notes                              |
|---------------------|-------------------------|------------|--------------------------------------|
| `--username`        | `VAULT_USERNAME`       | —          | prompted interactively if empty     |
| `--password`        | `VAULT_PASSWORD`       | —          | prompted (hidden, no echo) if empty — prefer the prompt over this flag/env on shared machines, since both are visible to other local processes (`ps`, `/proc`) |
| `--userpass-mount`  | `VAULT_USERPASS_MOUNT` | `userpass` | mount path of the userpass auth method |
| `--ldap-mount`      | `VAULT_LDAP_MOUNT`     | `ldap`     | mount path of the LDAP auth method  |

Logs in against Vault's `userpass` or `ldap` auth method with a
username/password pair.

On startup, regardless of method, the app first reuses a cached token from
`~/.vault-token` if it's still valid (same file the official `vault` CLI
uses, so tokens are shared between the two) before falling back to the
selected auth method's login flow. A fresh login's resulting token is
cached back to `~/.vault-token`.

## Keybindings

| Screen         | Keys                                                                 |
|----------------|-----------------------------------------------------------------------|
| Mounts         | `↑/↓` navigate • `enter` open • `/` filter • `q` quit                |
| Browse         | `enter` open • `n` new secret • `N` new folder • `e` edit (new version, secrets only) • `c` copy • `x` move (secrets or whole folders) • `r` rename • `d`/`D` soft-delete/destroy (secrets only) • `esc`/`backspace` up a level |
| Browse (copy/move armed) | `p` paste into current folder • `esc` cancel — navigation keys (`enter`, `backspace`/`h`, even switching mounts) work as normal so you can reach any destination |
| Secret view    | `m` show/hide values • `e` edit • `c` copy • `x` move • `v` version history • `r` rename • `d` soft-delete • `D` destroy • `esc` back |
| Version list   | `↑/↓` navigate • `enter` view that version (read-only) • `esc` back  |
| Editor         | type to edit • `tab`/`shift+tab` move field • `↑/↓` row • `ctrl+n` add row • `ctrl+x` remove row • `ctrl+s` save • `esc` cancel |
| Rename / new-name prompts | type • `enter` confirm • `esc` cancel                     |
| Confirm dialog | `y` confirm • `n`/`esc` cancel                                        |

`ctrl+c` quits from any screen, including mid-edit with a text field
focused — it's the one guaranteed way out. `q` also quits, but only on the
navigation screens (mounts/browse/secret view) and never while a field is
focused, so it stays typeable everywhere else. The `/` filter matches by
plain substring (not fuzzy), and while it has focus every keystroke goes to
the filter box, not to list shortcuts like `n`/`e`/`r`/`d`.

The help line on the browse screen only shows shortcuts that are actually
valid for whatever's currently selected — `e`/`d`/`D` disappear when a
folder is selected, since editing/deleting only apply to secrets. `r`
(rename) shows for both.

## Notable design decisions / limitations

- **Only KV v2 mounts are supported.** KV v1 mounts show up in the mount
  list but can't be opened — v1 has no version history, which is most of
  the point of this app.
- **Rename is copy + delete, not atomic.** Vault's KV v2 engine has no
  native rename. Renaming a secret or folder reads the current data, writes
  it to the new path (starting a fresh version history there), then deletes
  the old path's metadata. The old version history is lost — the app warns
  about this before you confirm. If a folder rename fails partway through,
  the error message tells you how many secrets were already moved so you
  can finish or roll back manually.
- **Rename only edits the leaf name, never the parent path.** The prompt
  shows the parent as a fixed, non-editable prefix — you can only type the
  new last segment. This is deliberate: since rename is really a move
  (copy+delete), letting the whole path be freely edited made it too easy
  to fat-finger a secret or an entire folder out to some unrelated location
  and wipe things by accident.
- **Values accept plain text or JSON.** The value field parses what you
  type as JSON first (numbers, `true`/`false`, `null`, objects, arrays);
  anything that isn't valid JSON is stored as a plain string. A live tag
  next to each row (`text`, `json·number`, `json·object`, ...) shows how
  it'll be saved. A string secret whose literal value happens to look like
  JSON (e.g. `"12345"`) is auto-quoted on display so re-saving it unedited
  can't silently flip its type — wrap a value in `"quotes"` yourself to
  force it to stay a string.
- **Secret values are masked by default in the detail view.** Every value
  shows as `••••••••` until you press `m`. It re-masks every time you open
  a secret or switch versions — the editor still shows real cleartext,
  since you need it to edit.
- **Copy (`c`) and move (`x`) work on secrets or whole folders, across
  mounts.** `c` or `x` on the selected entry arms it, then you browse —
  anywhere, including into a completely different KV v2 mount your token
  has access to (e.g. another client's) — and `p` pastes it into whatever
  folder you're looking at, under its original name. `esc` cancels. On a
  folder, this recurses: every secret found underneath is copied to the
  same relative path under the destination, and `x` only deletes the
  originals once every single one has copied successfully (if it fails
  partway through, the error says how many were moved so you can finish or
  roll back manually — same as folder rename). Pasting a folder into itself
  or one of its own subfolders is refused outright, since deleting the
  source afterwards would also delete the copy that just landed inside it.
  If the destination already has something at a matching path, you're asked
  to confirm before it's overwritten (as a new version there — the old data
  isn't destroyed, just superseded).
- **`d` vs `D`:** lowercase soft-deletes the latest version (recoverable
  via Vault's own CLI/API — this app doesn't expose undelete yet).
  Uppercase permanently destroys the secret and all of its version history.
  There is no folder-level bulk delete — that's deliberately left out to
  avoid an easy way to nuke a large subtree by accident.
- **"Empty folders" are a placeholder secret.** Vault KV v2 has no folder
  object — `LIST` only surfaces path prefixes that have at least one real
  secret underneath. `N` (new folder) works around this by writing a small
  `.keep` secret inside the new path, which is what makes it show up in
  listings. It's shown in the browser tagged "placeholder — safe to
  delete" and behaves like any other secret — delete it once you've added
  a real one, or leave it, it's harmless either way.

## Project layout

```
main.go                    flags/env wiring, auth bootstrap, launches the TUI
internal/vault/
  config.go                Config struct (addr, namespace, oidc mount/role)
  client.go                Vault API client construction + token caching
  oidc.go                  browser-based OIDC login flow
  kv.go                    KV v2 operations: list, read, write, delete, rename
internal/tui/
  model.go                 root Bubble Tea model, screen state machine
  mounts.go / browse.go / secret.go   read/navigate screens
  editor.go / newname.go / rename.go / confirm.go   write/mutate screens
  items.go                 list.Item wrappers for mounts/entries/versions
  messages.go               tea.Cmd constructors + result message types
  styles.go                 lipgloss styles
```

## Possible next steps

- Undelete / rollback UI for soft-deleted or older versions
- Custom metadata (labels, TTL) editing
- AppRole auth method, if you need it for automation/CI use cases
- Recursive folder delete (intentionally omitted for now — see above)
