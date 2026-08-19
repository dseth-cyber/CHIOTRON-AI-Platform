package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestS3ProviderPutGetDelete(t *testing.T) {
	var mu sync.Mutex
	objects := make(map[string][]byte)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		// Verify AWS SigV4 headers exist
		if r.Header.Get("x-amz-date") == "" {
			t.Errorf("missing x-amz-date header")
		}
		if r.Header.Get("Authorization") == "" {
			t.Errorf("missing Authorization header")
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		switch r.Method {
		case http.MethodPut:
			data, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			objects[path] = data
			w.WriteHeader(http.StatusOK)

		case http.MethodGet:
			data, ok := objects[path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)

		case http.MethodDelete:
			delete(objects, path)
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer ts.Close()

	s3, err := NewS3(S3Config{
		Endpoint:  ts.URL,
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		AccessKey: "test-access-key",
		SecretKey: "test-secret-key",
		PathStyle: true,
	})
	if err != nil {
		t.Fatalf("NewS3() error: %v", err)
	}

	if s3.Name() != "s3" {
		t.Errorf("Name() = %q, want 's3'", s3.Name())
	}

	ctx := context.Background()
	key := "tenant/2026/08/doc-123"
	payload := []byte("s3 content test")

	// 1. Put
	obj, err := s3.Put(ctx, key, payload)
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}
	if obj.Size != int64(len(payload)) {
		t.Errorf("Size = %d, want %d", obj.Size, len(payload))
	}
	if obj.Checksum != Checksum(payload) {
		t.Errorf("Checksum = %q, want %q", obj.Checksum, Checksum(payload))
	}

	// 2. Get
	fetched, err := s3.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if string(fetched) != string(payload) {
		t.Errorf("Get() = %q, want %q", string(fetched), string(payload))
	}

	// 3. Delete
	if err := s3.Delete(ctx, key); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// 4. Get after delete should return ErrNotFound
	if _, err := s3.Get(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() after Delete() = %v, want ErrNotFound", err)
	}
}

func TestNewS3Validation(t *testing.T) {
	if _, err := NewS3(S3Config{Bucket: ""}); err == nil {
		t.Error("NewS3() accepted empty bucket")
	}
}
