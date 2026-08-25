package rag

import (
	"bytes"
	"context"
	"testing"
)

func TestLoadExtractedTextDocument(t *testing.T) {
	documents, err := LoadExtractedDocuments(
		context.Background(),
		"document-id",
		"notes.md",
		"text/markdown",
		bytes.NewReader([]byte("# Evidence\nA source-backed statement.")),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 1 || documents[0].DocumentID != "document-id" {
		t.Fatalf("unexpected documents: %+v", documents)
	}
}
