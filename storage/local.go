package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("local object root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve local object root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, fmt.Errorf("create local object root: %w", err)
	}
	return &LocalStore{root: abs}, nil
}

func (s *LocalStore) resolve(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" || filepath.IsAbs(key) {
		return "", errors.New("invalid object key")
	}
	clean := filepath.Clean(filepath.FromSlash(key))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid object key")
	}
	path := filepath.Join(s.root, clean)
	rel, err := filepath.Rel(s.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("object key escapes storage root")
	}
	return path, nil
}

func (s *LocalStore) Put(ctx context.Context, key string, source io.Reader) (ObjectInfo, error) {
	path, err := s.resolve(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	if err := ctx.Err(); err != nil {
		return ObjectInfo{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return ObjectInfo{}, fmt.Errorf("create object directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("create temporary object: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(temporary, hash), source)
	closeErr := temporary.Close()
	if copyErr != nil {
		return ObjectInfo{}, fmt.Errorf("write object: %w", copyErr)
	}
	if closeErr != nil {
		return ObjectInfo{}, fmt.Errorf("close object: %w", closeErr)
	}
	if err := ctx.Err(); err != nil {
		return ObjectInfo{}, err
	}
	if err := os.Chmod(temporaryPath, 0o600); err != nil {
		return ObjectInfo{}, fmt.Errorf("secure object: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return ObjectInfo{}, fmt.Errorf("commit object: %w", err)
	}
	return ObjectInfo{SizeBytes: written, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func (s *LocalStore) Open(ctx context.Context, key string) (ReadSeekCloser, error) {
	path, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open object: %w", err)
	}
	return file, nil
}

func (s *LocalStore) Delete(ctx context.Context, key string) error {
	path, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}
