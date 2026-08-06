// Package vault wraps the parts of the Vault API this app needs: auth,
// and KV v2 browsing/editing (including the copy+delete dance that stands
// in for the rename Vault's KV v2 engine doesn't natively support).
package vault

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/vault/api"
)

// KeepFileName is the marker secret written inside an otherwise-empty
// folder so it shows up in listings. Vault KV v2 has no concept of a
// folder as such — LIST only surfaces path prefixes that have at least one
// real secret underneath them — so an "empty folder" is really just a
// folder containing nothing but this placeholder.
const KeepFileName = ".keep"

// Mount describes a mounted secrets engine relevant to this app.
type Mount struct {
	Path    string // e.g. "secret/"
	Type    string // e.g. "kv"
	Version string // KV engine version, "1" or "2"
}

// Entry is one item inside a folder listing: either a sub-folder or a leaf
// secret.
type Entry struct {
	Name     string
	IsFolder bool
}

// ListKVMounts returns every KV secrets engine mounted in Vault, regardless
// of version, so the UI can flag v1 mounts as read-only/unsupported for
// versioning features.
func ListKVMounts(ctx context.Context, client *api.Client) ([]Mount, error) {
	mounts, err := client.Sys().ListMountsWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing secret engines: %w", err)
	}

	var out []Mount
	for path, m := range mounts {
		if m.Type != "kv" {
			continue
		}
		version := "1"
		if v, ok := m.Options["version"]; ok && v != "" {
			version = v
		}
		out = append(out, Mount{Path: path, Type: m.Type, Version: version})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// ListPath lists the folders and secrets directly under path within mount
// (KV v2). path may be "" for the mount root. Trailing/leading slashes are
// normalized.
func ListPath(ctx context.Context, client *api.Client, mount, path string) ([]Entry, error) {
	path = strings.Trim(path, "/")
	metaPath := mount + "/metadata/"
	if path != "" {
		metaPath += path + "/"
	}

	secret, err := client.Logical().ListWithContext(ctx, metaPath)
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", metaPath, err)
	}
	if secret == nil || secret.Data == nil {
		return nil, nil
	}

	keysRaw, ok := secret.Data["keys"].([]interface{})
	if !ok {
		return nil, nil
	}

	entries := make([]Entry, 0, len(keysRaw))
	for _, k := range keysRaw {
		name, _ := k.(string)
		if name == "" {
			continue
		}
		entries = append(entries, Entry{
			Name:     strings.TrimSuffix(name, "/"),
			IsFolder: strings.HasSuffix(name, "/"),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsFolder != entries[j].IsFolder {
			return entries[i].IsFolder // folders first
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

// ReadSecret returns the key/value data for a secret. version == 0 means
// "latest".
func ReadSecret(ctx context.Context, client *api.Client, mount, path string, version int) (*api.KVSecret, error) {
	kv := client.KVv2(mount)
	if version == 0 {
		return kv.Get(ctx, path)
	}
	return kv.GetVersion(ctx, path, version)
}

// ListVersions returns the version history of a secret, newest first.
func ListVersions(ctx context.Context, client *api.Client, mount, path string) ([]api.KVVersionMetadata, error) {
	versions, err := client.KVv2(mount).GetVersionsAsList(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("listing versions for %s/%s: %w", mount, path, err)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i].Version > versions[j].Version })
	return versions, nil
}

// WriteSecret creates a new version of the secret at path with the given
// data (full replace of the key/value set — Vault KV v2 always versions
// the whole object, there's no partial-field diffing at the storage layer).
func WriteSecret(ctx context.Context, client *api.Client, mount, path string, data map[string]interface{}) (*api.KVSecret, error) {
	secret, err := client.KVv2(mount).Put(ctx, path, data)
	if err != nil {
		return nil, fmt.Errorf("writing %s/%s: %w", mount, path, err)
	}
	return secret, nil
}

// CreateFolder makes an otherwise-empty folder show up in listings by
// writing KeepFileName as a placeholder secret inside it. It's a normal
// secret like any other — safe to delete (or leave) once you've added a
// real secret under the same prefix.
func CreateFolder(ctx context.Context, client *api.Client, mount, path string) error {
	keepPath := strings.Trim(path, "/") + "/" + KeepFileName
	if _, err := WriteSecret(ctx, client, mount, keepPath, map[string]interface{}{
		"note": "placeholder so this folder shows up in listings — safe to delete once you add a real secret here",
	}); err != nil {
		return fmt.Errorf("creating folder %s/%s: %w", mount, path, err)
	}
	return nil
}

// DeleteLatestVersion soft-deletes the current version of a secret (data
// stays recoverable via Undelete until it's destroyed or GC'd).
func DeleteLatestVersion(ctx context.Context, client *api.Client, mount, path string) error {
	if err := client.KVv2(mount).Delete(ctx, path); err != nil {
		return fmt.Errorf("deleting %s/%s: %w", mount, path, err)
	}
	return nil
}

// DeleteMetadata permanently removes a secret and all of its version
// history. There is no undo.
func DeleteMetadata(ctx context.Context, client *api.Client, mount, path string) error {
	if err := client.KVv2(mount).DeleteMetadata(ctx, path); err != nil {
		return fmt.Errorf("deleting metadata for %s/%s: %w", mount, path, err)
	}
	return nil
}

// SecretExists reports whether a secret currently exists at mount/path,
// distinguishing "not found" (false, nil) from a real error.
func SecretExists(ctx context.Context, client *api.Client, mount, path string) (bool, error) {
	_, err := client.KVv2(mount).Get(ctx, path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, api.ErrSecretNotFound) {
		return false, nil
	}
	return false, fmt.Errorf("checking %s/%s: %w", mount, path, err)
}

// CopySecret reads the current data at srcMount/srcPath and writes it to
// dstMount/dstPath (which may be the same mount, a different folder, or an
// entirely different KV v2 mount — e.g. one belonging to a different
// client, if your token has access to it). The source is left untouched.
// Like every other write in this app, the destination gets a fresh version
// history — Vault has no way to carry version history across paths.
func CopySecret(ctx context.Context, client *api.Client, srcMount, srcPath, dstMount, dstPath string) error {
	current, err := ReadSecret(ctx, client, srcMount, srcPath, 0)
	if err != nil {
		return fmt.Errorf("reading %s/%s: %w", srcMount, srcPath, err)
	}
	if current == nil {
		return fmt.Errorf("%s/%s not found", srcMount, srcPath)
	}
	if _, err := WriteSecret(ctx, client, dstMount, dstPath, current.Data); err != nil {
		return fmt.Errorf("writing %s/%s: %w", dstMount, dstPath, err)
	}
	return nil
}

// MoveSecret is CopySecret followed by permanently removing the source.
func MoveSecret(ctx context.Context, client *api.Client, srcMount, srcPath, dstMount, dstPath string) error {
	if err := CopySecret(ctx, client, srcMount, srcPath, dstMount, dstPath); err != nil {
		return err
	}
	if err := DeleteMetadata(ctx, client, srcMount, srcPath); err != nil {
		return fmt.Errorf("copied to %s/%s but failed to remove source %s/%s: %w", dstMount, dstPath, srcMount, srcPath, err)
	}
	return nil
}

// RenameSecret moves a single secret from oldPath to newPath within the
// same mount. Vault's KV v2 engine has no atomic rename primitive, so this
// is just MoveSecret with the same mount on both ends — see MoveSecret for
// the copy+delete mechanics and version-history caveat.
func RenameSecret(ctx context.Context, client *api.Client, mount, oldPath, newPath string) error {
	return MoveSecret(ctx, client, mount, oldPath, mount, newPath)
}

// ListAllSecretsRecursive walks prefix and returns the full paths of every
// leaf secret found underneath it (folders are traversed, not returned).
func ListAllSecretsRecursive(ctx context.Context, client *api.Client, mount, prefix string) ([]string, error) {
	var out []string
	entries, err := ListPath(ctx, client, mount, prefix)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		full := strings.Trim(prefix, "/")
		if full != "" {
			full += "/"
		}
		full += e.Name

		if e.IsFolder {
			sub, err := ListAllSecretsRecursive(ctx, client, mount, full)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
		} else {
			out = append(out, full)
		}
	}
	return out, nil
}

// FolderHasContent reports whether prefix currently has anything directly
// underneath it. Used to warn before a folder copy/move might overwrite
// data already sitting at the destination — it's a coarse check (it doesn't
// diff individual secret paths), same spirit as SecretExists for a single
// secret.
func FolderHasContent(ctx context.Context, client *api.Client, mount, prefix string) (bool, error) {
	entries, err := ListPath(ctx, client, mount, prefix)
	if err != nil {
		return false, fmt.Errorf("checking %s/%s: %w", mount, prefix, err)
	}
	return len(entries) > 0, nil
}

// CopyFolder copies every secret found under srcPrefix to the same
// relative location under dstPrefix, which may be a different mount
// entirely. The source is left untouched. Each destination secret gets a
// fresh version history, same as CopySecret.
func CopyFolder(ctx context.Context, client *api.Client, srcMount, srcPrefix, dstMount, dstPrefix string) error {
	srcPrefix = strings.Trim(srcPrefix, "/")
	dstPrefix = strings.Trim(dstPrefix, "/")

	paths, err := ListAllSecretsRecursive(ctx, client, srcMount, srcPrefix)
	if err != nil {
		return fmt.Errorf("listing %s/%s: %w", srcMount, srcPrefix, err)
	}
	if len(paths) == 0 {
		return fmt.Errorf("no secrets found under %s/%s", srcMount, srcPrefix)
	}

	for i, srcPath := range paths {
		rel := strings.TrimPrefix(srcPath, srcPrefix+"/")
		dstPath := dstPrefix + "/" + rel
		if err := CopySecret(ctx, client, srcMount, srcPath, dstMount, dstPath); err != nil {
			return fmt.Errorf("copied %d/%d secrets before failing on %s/%s: %w", i, len(paths), srcMount, srcPath, err)
		}
	}
	return nil
}

// MoveFolder is CopyFolder followed by permanently removing every source
// secret once the whole copy has succeeded. RenameFolder is just this with
// srcMount == dstMount. Same version-history caveat as MoveSecret applies
// to each secret moved.
func MoveFolder(ctx context.Context, client *api.Client, srcMount, srcPrefix, dstMount, dstPrefix string) error {
	srcPrefix = strings.Trim(srcPrefix, "/")
	dstPrefix = strings.Trim(dstPrefix, "/")

	paths, err := ListAllSecretsRecursive(ctx, client, srcMount, srcPrefix)
	if err != nil {
		return fmt.Errorf("listing %s/%s: %w", srcMount, srcPrefix, err)
	}
	if len(paths) == 0 {
		return fmt.Errorf("no secrets found under %s/%s", srcMount, srcPrefix)
	}

	var moved []string
	for _, srcPath := range paths {
		rel := strings.TrimPrefix(srcPath, srcPrefix+"/")
		dstPath := dstPrefix + "/" + rel
		if err := CopySecret(ctx, client, srcMount, srcPath, dstMount, dstPath); err != nil {
			return fmt.Errorf("moved %d/%d secrets before failing to copy %s/%s: %w", len(moved), len(paths), srcMount, srcPath, err)
		}
		moved = append(moved, srcPath)
	}

	for _, srcPath := range moved {
		if err := DeleteMetadata(ctx, client, srcMount, srcPath); err != nil {
			return fmt.Errorf("copied all secrets to %s/%s but failed cleaning up old path %s/%s: %w", dstMount, dstPrefix, srcMount, srcPath, err)
		}
	}
	return nil
}

// RenameFolder moves every secret found under oldPrefix to the same
// relative location under newPrefix within the same mount, then removes
// the old ones. See MoveFolder for the copy+delete mechanics and
// version-history caveat.
func RenameFolder(ctx context.Context, client *api.Client, mount, oldPrefix, newPrefix string) error {
	return MoveFolder(ctx, client, mount, oldPrefix, mount, newPrefix)
}
