package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newLocal(t *testing.T) *Local {
	t.Helper()
	local, err := NewLocal(filepath.Join(t.TempDir(), "documents"))
	if err != nil {
		t.Fatalf("NewLocal() returned error: %v", err)
	}
	return local
}

func TestPutGetDelete(t *testing.T) {
	local := newLocal(t)
	ctx := context.Background()
	content := []byte("hello corpus")

	object, err := local.Put(ctx, "acme/2026/08/doc-1", content)
	if err != nil {
		t.Fatalf("Put() returned error: %v", err)
	}
	if object.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", object.Size, len(content))
	}
	if object.Checksum != Checksum(content) {
		t.Errorf("Checksum = %q, want the content address", object.Checksum)
	}

	read, err := local.Get(ctx, object.Key)
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if string(read) != string(content) {
		t.Errorf("Get() = %q, want %q", read, content)
	}

	if err := local.Delete(ctx, object.Key); err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}
	if _, err := local.Get(ctx, object.Key); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() after Delete returned %v, want ErrNotFound", err)
	}
}

// Deleting something that is already gone is how cleanup after a partial failure
// behaves, so it must not be an error.
func TestDeleteIsIdempotent(t *testing.T) {
	local := newLocal(t)

	if err := local.Delete(context.Background(), "never/existed"); err != nil {
		t.Fatalf("Delete() on a missing key returned %v, want nil", err)
	}
}

// A key that escaped the root would expose the whole filesystem, and an
// absolute key silently rebased under it would hide a caller bug.
func TestResolveRefusesTraversalAndAbsoluteKeys(t *testing.T) {
	local := newLocal(t)
	ctx := context.Background()

	for _, key := range []string{"../escape", "acme/../../escape", "/etc/passwd", `\windows\system32`, ""} {
		if _, err := local.Put(ctx, key, []byte("x")); err == nil {
			t.Errorf("Put(%q) succeeded, want a refusal", key)
		}
	}
}

// A crash mid-write must not leave a truncated object behind a valid key.
func TestPutLeavesNoPartialFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "documents")
	local, err := NewLocal(root)
	if err != nil {
		t.Fatalf("NewLocal() returned error: %v", err)
	}

	if _, err := local.Put(context.Background(), "a/b/c", []byte("payload")); err != nil {
		t.Fatalf("Put() returned error: %v", err)
	}

	var partials []string
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() && strings.HasSuffix(path, ".partial") {
			partials = append(partials, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk storage root: %v", err)
	}
	if len(partials) != 0 {
		t.Errorf("temporary files survived the write: %v", partials)
	}
}

func TestKeySpreadsAndKeepsTenantVisible(t *testing.T) {
	key := Key("Acme Corp", "9f8e7d6c")

	if !strings.HasPrefix(key, "acme-corp/") {
		t.Errorf("key = %q, want the tenant as a lowercase leading segment", key)
	}
	if strings.Count(key, "/") != 3 {
		t.Errorf("key = %q, want tenant/year/month/id", key)
	}
}

// A document with no company still needs somewhere to live. The underscore
// prefix marks the directory as platform-level rather than a tenant.
func TestKeyHandlesMissingCompany(t *testing.T) {
	key := Key("", "abc")

	if !strings.HasPrefix(key, "_platform/") {
		t.Errorf("key = %q, want the platform-level prefix", key)
	}
}

func TestNewLocalRequiresRoot(t *testing.T) {
	if _, err := NewLocal("   "); err == nil {
		t.Fatal("NewLocal() accepted a blank root, want error")
	}
}
