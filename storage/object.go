package storage

import (
	"context"
	"io"
)

// ReadSeekCloser is the smallest object handle needed by the document loaders.
// Local files satisfy it directly; a future remote adapter may use a temporary file.
type ReadSeekCloser interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Closer
}

type ObjectInfo struct {
	SizeBytes int64
	SHA256    string
}

// ObjectStore owns source files only. Embeddings and searchable metadata live
// in PostgreSQL, so swapping this adapter cannot silently change retrieval.
type ObjectStore interface {
	Put(ctx context.Context, key string, source io.Reader) (ObjectInfo, error)
	Open(ctx context.Context, key string) (ReadSeekCloser, error)
	Delete(ctx context.Context, key string) error
}
