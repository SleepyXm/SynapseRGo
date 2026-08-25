package rag

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var ErrOCRRequired = errors.New("PDF contains no extractable text; OCR is required")

// LoadExtractedDocuments converts a stored upload into the common Document
// boundary. PDF pages remain separate so citations retain page identity.
func LoadExtractedDocuments(
	ctx context.Context,
	documentID string,
	filename string,
	mediaType string,
	source interface {
		io.Reader
		io.ReaderAt
		io.Seeker
	},
) ([]Document, error) {
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind source: %w", err)
	}
	extension := strings.ToLower(filepath.Ext(filename))
	isPDF := mediaType == "application/pdf" || extension == ".pdf"
	if !isPDF {
		content, err := io.ReadAll(source)
		if err != nil {
			return nil, fmt.Errorf("extract text: %w", err)
		}
		if strings.TrimSpace(string(content)) == "" {
			return nil, errors.New("document contains no text")
		}
		return []Document{{
			ID:         documentID,
			DocumentID: documentID,
			Source:     filename,
			Content:    string(content),
			Metadata:   map[string]string{"media_type": mediaType},
		}}, nil
	}

	pdfToText, err := exec.LookPath("pdftotext")
	if err != nil {
		return nil, errors.New("PDF extraction requires pdftotext (Poppler) to be installed")
	}
	temporary, err := os.CreateTemp("", "synapse-document-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("create temporary PDF: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		temporary.Close()
		return nil, fmt.Errorf("rewind PDF: %w", err)
	}
	if _, err := io.Copy(temporary, source); err != nil {
		temporary.Close()
		return nil, fmt.Errorf("stage PDF: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return nil, fmt.Errorf("close staged PDF: %w", err)
	}
	output, err := exec.CommandContext(ctx, pdfToText, "-layout", "-enc", "UTF-8", temporaryPath, "-").Output()
	if err != nil {
		return nil, fmt.Errorf("extract PDF text: %w", err)
	}
	pages := strings.Split(string(output), "\f")
	documents := make([]Document, 0, len(pages))
	for index, page := range pages {
		if strings.TrimSpace(page) == "" {
			continue
		}
		pageNumber := index + 1
		documents = append(documents, Document{
			ID:         fmt.Sprintf("%s:page:%d", documentID, pageNumber),
			DocumentID: documentID,
			Source:     filename,
			Content:    page,
			Metadata: map[string]string{
				"media_type": mediaType,
				"page":       strconv.Itoa(pageNumber),
			},
		})
	}
	if len(documents) == 0 {
		return nil, ErrOCRRequired
	}
	return documents, nil
}
