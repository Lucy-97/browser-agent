package artifact

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStoreLifecycle(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalStore(root)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := store.Put(context.Background(), PutInput{
		TenantID:  "tenant/a",
		RunID:     "run/1",
		Filename:  "../trace.json",
		SizeBytes: 7,
		Body:      strings.NewReader("payload"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored.Key, "..") || !strings.HasSuffix(stored.Key, "trace.json") {
		t.Fatalf("unsafe storage key: %q", stored.Key)
	}

	object, err := store.Get(context.Background(), stored.Key, "")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(object.Body)
	object.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, []byte("payload")) || object.ContentLength != 7 {
		t.Fatalf("unexpected object: content=%q size=%d", raw, object.ContentLength)
	}

	if err := store.Delete(context.Background(), stored.Key); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), stored.Key, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() after delete error = %v, want ErrNotFound", err)
	}
}

func TestLocalStoreRejectsEscapingKey(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), "../../outside", ""); err == nil {
		t.Fatal("expected path traversal rejection")
	}
}

func TestLocalStoreReadsLegacyAbsoluteKeyInsideRoot(t *testing.T) {
	root := t.TempDir()
	legacyPath := filepath.Join(root, "legacy", "artifact.txt")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewLocalStore(root)
	if err != nil {
		t.Fatal(err)
	}
	object, err := store.Get(context.Background(), legacyPath, "")
	if err != nil {
		t.Fatal(err)
	}
	defer object.Body.Close()
	raw, err := io.ReadAll(object.Body)
	if err != nil || string(raw) != "legacy" {
		t.Fatalf("legacy object = %q, %v", raw, err)
	}
}

func TestLocalStoreRemovesPartialSizeMismatch(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalStore(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Put(context.Background(), PutInput{
		TenantID: "tenant", RunID: "run", Filename: "short.txt",
		SizeBytes: 10, Body: strings.NewReader("short"),
	})
	if err == nil {
		t.Fatal("expected size mismatch")
	}
	files := 0
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr == nil && info != nil && !info.IsDir() {
			files++
		}
		return nil
	})
	if files != 0 {
		t.Fatalf("partial artifact files = %d, want 0", files)
	}
}

func TestSanitizeFilename(t *testing.T) {
	if got := SanitizeFilename(`..\folder/报告 2026?.pdf`); got != "报告-2026-.pdf" {
		t.Fatalf("SanitizeFilename() = %q", got)
	}
}
