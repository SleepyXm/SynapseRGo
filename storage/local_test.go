package storage

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestLocalStoreRoundTripAndRejectsTraversal(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	info, err := store.Put(context.Background(), "users/u/knowledge/k/d", strings.NewReader("source"))
	if err != nil {
		t.Fatal(err)
	}
	if info.SizeBytes != 6 || len(info.SHA256) != 64 {
		t.Fatalf("unexpected object info: %+v", info)
	}
	object, err := store.Open(context.Background(), "users/u/knowledge/k/d")
	if err != nil {
		t.Fatal(err)
	}
	defer object.Close()
	content, err := io.ReadAll(object)
	if err != nil || string(content) != "source" {
		t.Fatalf("content = %q, error = %v", content, err)
	}
	if _, err := store.Put(context.Background(), "../escape", strings.NewReader("bad")); err == nil {
		t.Fatal("expected traversal key to fail")
	}
}
