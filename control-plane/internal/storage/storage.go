// Package storage abstracts where document bytes live.
//
// StorageProvider is one of the replaceable adapters in ARCHITECTURE-v1
// section 4: local disk during development, NAS, S3 or MinIO in production. The
// production provider of record is still an open decision (section 13 item 5),
// which is exactly why nothing above this package may name one.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrNotFound = errors.New("stored object not found")

// Object is what a Put returns: enough to find the bytes again and to recognise
// the same bytes arriving twice.
type Object struct {
	Key      string
	Size     int64
	Checksum string
}

type Provider interface {
	// Name identifies the adapter in logs and health output.
	Name() string
	Put(ctx context.Context, key string, content []byte) (Object, error)
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// Checksum is the content address used to deduplicate uploads.
func Checksum(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// Key builds a storage key that spreads documents across directories and keeps
// the tenant visible, so a misplaced object is recognisable on sight.
func Key(companyID, documentID string) string {
	tenant := companyID
	if tenant == "" {
		tenant = "_platform"
	}
	now := time.Now().UTC()
	return fmt.Sprintf("%s/%04d/%02d/%s", sanitise(tenant), now.Year(), int(now.Month()), sanitise(documentID))
}

// Local stores objects on a filesystem path. In production this is a NAS mount
// or is replaced by an object-store adapter.
type Local struct {
	root string
}

func NewLocal(root string) (*Local, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("storage root is required")
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("create storage root %q: %w", root, err)
	}
	return &Local{root: root}, nil
}

func (l *Local) Name() string { return "local" }

func (l *Local) Put(_ context.Context, key string, content []byte) (Object, error) {
	path, err := l.resolve(key)
	if err != nil {
		return Object{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return Object{}, fmt.Errorf("create object directory: %w", err)
	}

	// Write to a temporary name and rename, so a crash cannot leave a
	// half-written object behind a valid-looking key.
	temporary := path + ".partial"
	if err := os.WriteFile(temporary, content, 0o640); err != nil {
		return Object{}, fmt.Errorf("write object: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return Object{}, fmt.Errorf("commit object: %w", err)
	}

	return Object{Key: key, Size: int64(len(content)), Checksum: Checksum(content)}, nil
}

func (l *Local) Get(_ context.Context, key string) ([]byte, error) {
	path, err := l.resolve(key)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, fmt.Errorf("%w: %q", ErrNotFound, key)
	case err != nil:
		return nil, fmt.Errorf("read object: %w", err)
	}
	return content, nil
}

func (l *Local) Delete(_ context.Context, key string) error {
	path, err := l.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

// resolve refuses any key that would escape the root. Keys are derived from
// identifiers rather than user input today, but a traversal here would expose
// the whole filesystem, so it is checked rather than assumed.
func (l *Local) resolve(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("object key is required")
	}
	// An absolute key would be joined silently under the root, which is safe but
	// surprising: `/etc/passwd` becoming `<root>/etc/passwd` hides a caller bug.
	// Keys are relative by contract, so say so.
	if strings.HasPrefix(key, "/") || strings.HasPrefix(key, `\`) || filepath.IsAbs(key) {
		return "", fmt.Errorf("object key %q must be relative", key)
	}
	path := filepath.Join(l.root, filepath.FromSlash(key))
	cleanRoot := filepath.Clean(l.root)
	if path != cleanRoot && !strings.HasPrefix(path, cleanRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("object key %q escapes the storage root", key)
	}
	return path, nil
}

// sanitise keeps a path segment to characters that are safe on every target,
// including Windows development hosts and object stores.
func sanitise(segment string) string {
	var builder strings.Builder
	for _, char := range segment {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9', char == '-', char == '_':
			builder.WriteRune(char)
		case char >= 'A' && char <= 'Z':
			builder.WriteRune(char + 32)
		default:
			builder.WriteByte('-')
		}
	}
	cleaned := strings.Trim(builder.String(), "-")
	if cleaned == "" {
		return "object"
	}
	return cleaned
}
