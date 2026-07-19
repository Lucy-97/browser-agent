package artifact

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type LocalStore struct {
	root string
}

func NewLocalStore(root string) (*LocalStore, error) {
	if strings.TrimSpace(root) == "" {
		root = "artifacts"
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve artifact directory: %w", err)
	}
	return &LocalStore{root: absRoot}, nil
}

func (store *LocalStore) Put(ctx context.Context, input PutInput) (StoredObject, error) {
	if input.Body == nil {
		return StoredObject{}, fmt.Errorf("artifact body is required")
	}
	key := objectKey("", input.TenantID, input.RunID, input.Filename)
	filename, err := store.resolve(key)
	if err != nil {
		return StoredObject{}, err
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		return StoredObject{}, fmt.Errorf("create artifact path: %w", err)
	}
	output, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return StoredObject{}, fmt.Errorf("create artifact object: %w", err)
	}
	written, copyErr := io.Copy(output, contextReader{ctx: ctx, reader: input.Body})
	closeErr := output.Close()
	if copyErr != nil || closeErr != nil || (input.SizeBytes >= 0 && written != input.SizeBytes) {
		_ = os.Remove(filename)
		switch {
		case copyErr != nil:
			return StoredObject{}, fmt.Errorf("write artifact object: %w", copyErr)
		case closeErr != nil:
			return StoredObject{}, fmt.Errorf("close artifact object: %w", closeErr)
		default:
			return StoredObject{}, fmt.Errorf("artifact size mismatch: wrote %d bytes, expected %d", written, input.SizeBytes)
		}
	}
	return StoredObject{Key: key}, nil
}

func (store *LocalStore) Get(ctx context.Context, key string, _ string) (Object, error) {
	if err := ctx.Err(); err != nil {
		return Object{}, err
	}
	filename, err := store.resolve(key)
	if err != nil {
		return Object{}, err
	}
	file, err := os.Open(filename)
	if errors.Is(err, os.ErrNotExist) {
		return Object{}, ErrNotFound
	}
	if err != nil {
		return Object{}, fmt.Errorf("open artifact object: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return Object{}, fmt.Errorf("stat artifact object: %w", err)
	}
	return Object{Body: file, ContentLength: info.Size(), LastModified: info.ModTime()}, nil
}

func (store *LocalStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	filename, err := store.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(filename); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete artifact object: %w", err)
	}
	return nil
}

func (store *LocalStore) resolve(key string) (string, error) {
	var candidate string
	if filepath.IsAbs(key) {
		candidate = filepath.Clean(key)
	} else {
		candidate = filepath.Join(store.root, filepath.FromSlash(key))
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve artifact key: %w", err)
	}
	relative, err := filepath.Rel(store.root, absCandidate)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("artifact key escapes configured storage root")
	}
	return absCandidate, nil
}
