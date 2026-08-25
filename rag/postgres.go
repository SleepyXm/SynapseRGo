package rag

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

const knowledgeBaseSelect = `
SELECT kb.id, kb.user_id, kb.name, kb.description, kb.embedding_model_id,
       kb.hf_token_name, kb.embedding_dimension, kb.chunk_size_runes,
       kb.chunk_overlap_runes, kb.created_at, kb.updated_at,
       COUNT(d.id) FILTER (WHERE d.status = 'ready')
FROM knowledge_bases kb
LEFT JOIN knowledge_documents d ON d.knowledge_base_id = kb.id
`

func scanKnowledgeBase(scanner interface{ Scan(...any) error }) (KnowledgeBase, error) {
	var base KnowledgeBase
	err := scanner.Scan(
		&base.ID, &base.UserID, &base.Name, &base.Description,
		&base.EmbeddingModelID, &base.HFTokenName, &base.EmbeddingDimension,
		&base.ChunkSizeRunes, &base.ChunkOverlapRunes, &base.CreatedAt,
		&base.UpdatedAt, &base.ReadyDocuments,
	)
	return base, err
}

func (r *PostgresRepository) CreateKnowledgeBase(ctx context.Context, base KnowledgeBase) (KnowledgeBase, error) {
	base.ID = uuid.NewString()
	_, err := r.db.ExecContext(ctx, `
INSERT INTO knowledge_bases
    (id, user_id, name, description, embedding_model_id, hf_token_name, chunk_size_runes, chunk_overlap_runes)
VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, $8)`,
		base.ID, base.UserID, base.Name, base.Description, base.EmbeddingModelID,
		base.HFTokenName, base.ChunkSizeRunes, base.ChunkOverlapRunes,
	)
	if err != nil {
		return KnowledgeBase{}, err
	}
	return r.GetKnowledgeBase(ctx, base.UserID, base.ID)
}

func (r *PostgresRepository) ListKnowledgeBases(ctx context.Context, userID string) ([]KnowledgeBase, error) {
	rows, err := r.db.QueryContext(ctx, knowledgeBaseSelect+`
WHERE kb.user_id = $1::uuid
GROUP BY kb.id
ORDER BY kb.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bases := []KnowledgeBase{}
	for rows.Next() {
		base, err := scanKnowledgeBase(rows)
		if err != nil {
			return nil, err
		}
		bases = append(bases, base)
	}
	return bases, rows.Err()
}

func (r *PostgresRepository) GetKnowledgeBase(ctx context.Context, userID, id string) (KnowledgeBase, error) {
	return scanKnowledgeBase(r.db.QueryRowContext(ctx, knowledgeBaseSelect+`
WHERE kb.id = $1::uuid AND kb.user_id = $2::uuid
GROUP BY kb.id`, id, userID))
}

func (r *PostgresRepository) GetKnowledgeBaseInternal(ctx context.Context, id string) (KnowledgeBase, error) {
	return scanKnowledgeBase(r.db.QueryRowContext(ctx, knowledgeBaseSelect+`
WHERE kb.id = $1::uuid
GROUP BY kb.id`, id))
}

func (r *PostgresRepository) UpdateKnowledgeBase(ctx context.Context, userID, id, name, description string) (KnowledgeBase, error) {
	result, err := r.db.ExecContext(ctx, `
UPDATE knowledge_bases SET name = $1, description = $2, updated_at = NOW()
WHERE id = $3::uuid AND user_id = $4::uuid`, name, description, id, userID)
	if err != nil {
		return KnowledgeBase{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return KnowledgeBase{}, sql.ErrNoRows
	}
	return r.GetKnowledgeBase(ctx, userID, id)
}

func (r *PostgresRepository) DeleteKnowledgeBase(ctx context.Context, userID, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM knowledge_bases WHERE id = $1::uuid AND user_id = $2::uuid`, id, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func scanDocument(scanner interface{ Scan(...any) error }) (DocumentRecord, error) {
	var document DocumentRecord
	err := scanner.Scan(&document.ID, &document.KnowledgeBaseID, &document.Filename,
		&document.MediaType, &document.ObjectKey, &document.SHA256, &document.SizeBytes,
		&document.Status, &document.FailureReason, &document.CreatedAt, &document.UpdatedAt)
	return document, err
}

const documentSelect = `SELECT d.id, d.knowledge_base_id, d.filename, d.media_type,
d.object_key, d.sha256, d.size_bytes, d.status, d.failure_reason, d.created_at, d.updated_at
FROM knowledge_documents d`

func (r *PostgresRepository) CreateDocument(ctx context.Context, document DocumentRecord) (DocumentRecord, error) {
	if document.ID == "" {
		document.ID = uuid.NewString()
	}
	return scanDocument(r.db.QueryRowContext(ctx, `
INSERT INTO knowledge_documents
    (id, knowledge_base_id, filename, media_type, object_key, sha256, size_bytes, status)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, 'queued')
RETURNING id, knowledge_base_id, filename, media_type, object_key, sha256,
          size_bytes, status, failure_reason, created_at, updated_at`,
		document.ID, document.KnowledgeBaseID, document.Filename, document.MediaType,
		document.ObjectKey, document.SHA256, document.SizeBytes,
	))
}

func (r *PostgresRepository) ListDocuments(ctx context.Context, userID, knowledgeBaseID string) ([]DocumentRecord, error) {
	rows, err := r.db.QueryContext(ctx, documentSelect+`
JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
WHERE d.knowledge_base_id = $1::uuid AND kb.user_id = $2::uuid
ORDER BY d.created_at DESC`, knowledgeBaseID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	documents := []DocumentRecord{}
	for rows.Next() {
		document, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}
	return documents, rows.Err()
}

func (r *PostgresRepository) GetDocument(ctx context.Context, userID, knowledgeBaseID, documentID string) (DocumentRecord, error) {
	return scanDocument(r.db.QueryRowContext(ctx, documentSelect+`
JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id
WHERE d.id = $1::uuid AND d.knowledge_base_id = $2::uuid AND kb.user_id = $3::uuid`,
		documentID, knowledgeBaseID, userID))
}

func (r *PostgresRepository) DeleteDocument(ctx context.Context, userID, knowledgeBaseID, documentID string) (DocumentRecord, error) {
	document, err := r.GetDocument(ctx, userID, knowledgeBaseID, documentID)
	if err != nil {
		return DocumentRecord{}, err
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM knowledge_documents WHERE id = $1::uuid AND knowledge_base_id = $2::uuid`, documentID, knowledgeBaseID)
	if err != nil {
		return DocumentRecord{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return DocumentRecord{}, sql.ErrNoRows
	}
	return document, nil
}

func (r *PostgresRepository) RetryDocument(ctx context.Context, userID, knowledgeBaseID, documentID string) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE knowledge_documents d SET status = 'queued', failure_reason = NULL, updated_at = NOW()
FROM knowledge_bases kb
WHERE d.id = $1::uuid AND d.knowledge_base_id = $2::uuid
  AND kb.id = d.knowledge_base_id AND kb.user_id = $3::uuid AND d.status = 'failed'`,
		documentID, knowledgeBaseID, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *PostgresRepository) RecoverProcessingDocuments(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `UPDATE knowledge_documents SET status = 'queued', updated_at = NOW() WHERE status = 'processing'`)
	return err
}

func (r *PostgresRepository) ClaimNextDocument(ctx context.Context) (DocumentRecord, error) {
	document, err := scanDocument(r.db.QueryRowContext(ctx, `
WITH next AS (
    SELECT id FROM knowledge_documents
    WHERE status = 'queued'
    ORDER BY created_at
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE knowledge_documents d SET status = 'processing', failure_reason = NULL, updated_at = NOW()
FROM next
WHERE d.id = next.id
RETURNING d.id, d.knowledge_base_id, d.filename, d.media_type, d.object_key,
          d.sha256, d.size_bytes, d.status, d.failure_reason, d.created_at, d.updated_at`))
	if errors.Is(err, sql.ErrNoRows) {
		return DocumentRecord{}, ErrNoQueuedDocuments
	}
	return document, err
}

func (r *PostgresRepository) SavePreparedDocument(ctx context.Context, base KnowledgeBase, document DocumentRecord, chunks []EmbeddedChunk) error {
	if len(chunks) == 0 || len(chunks[0].Vector) == 0 {
		return errors.New("prepared document contains no embeddings")
	}
	dimension := len(chunks[0].Vector)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
UPDATE knowledge_bases
SET embedding_dimension = COALESCE(embedding_dimension, $1), updated_at = NOW()
WHERE id = $2::uuid AND (embedding_dimension IS NULL OR embedding_dimension = $1)`, dimension, base.ID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("embedding dimension %d does not match knowledge base", dimension)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM knowledge_chunks WHERE document_id = $1::uuid`, document.ID); err != nil {
		return err
	}
	for _, chunk := range chunks {
		metadata, err := json.Marshal(chunk.Metadata)
		if err != nil {
			return err
		}
		var page *int
		if raw := strings.TrimSpace(chunk.Metadata["page"]); raw != "" {
			if value, parseErr := strconv.Atoi(raw); parseErr == nil {
				page = &value
			}
		}
		vector, err := vectorLiteral(chunk.Vector)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO knowledge_chunks
    (id, knowledge_base_id, document_id, chunk_index, page_number, content, metadata, embedding_model_id, embedding)
VALUES ($1, $2::uuid, $3::uuid, $4, $5, $6, $7::jsonb, $8, $9::vector)`,
			chunk.ID, base.ID, document.ID, chunk.Index, page, chunk.Content,
			string(metadata), chunk.EmbeddingModel, vector,
		)
		if err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE knowledge_documents SET status = 'ready', failure_reason = NULL, updated_at = NOW()
WHERE id = $1::uuid`, document.ID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *PostgresRepository) FailDocument(ctx context.Context, documentID, reason string) error {
	if len(reason) > 1000 {
		reason = reason[:1000]
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE knowledge_documents SET status = 'failed', failure_reason = $1, updated_at = NOW()
WHERE id = $2::uuid`, reason, documentID)
	return err
}

func (r *PostgresRepository) Search(ctx context.Context, userID, knowledgeBaseID string, vector []float32, limit int) ([]SearchResult, error) {
	literal, err := vectorLiteral(vector)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT c.id, c.document_id, d.filename, c.page_number, c.chunk_index, c.content,
       1 - (c.embedding <=> $3::vector) AS score, c.knowledge_base_id
FROM knowledge_chunks c
JOIN knowledge_documents d ON d.id = c.document_id AND d.status = 'ready'
JOIN knowledge_bases kb ON kb.id = c.knowledge_base_id
WHERE c.knowledge_base_id = $1::uuid AND kb.user_id = $2::uuid
ORDER BY c.embedding <=> $3::vector
LIMIT $4`, knowledgeBaseID, userID, literal, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []SearchResult{}
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.CitationID, &result.DocumentID, &result.Filename,
			&result.Page, &result.ChunkIndex, &result.Content, &result.Score,
			&result.KnowledgeID); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

// vectorLiteral uses PostgreSQL's native vector input format. Values remain a
// bound SQL parameter; they are never interpolated into a query string.
func vectorLiteral(values []float32) (string, error) {
	if len(values) == 0 {
		return "", errors.New("vector cannot be empty")
	}
	var builder strings.Builder
	builder.WriteByte('[')
	for index, value := range values {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return "", errors.New("vector contains a non-finite value")
		}
		if index > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(strconv.FormatFloat(float64(value), 'g', -1, 32))
	}
	builder.WriteByte(']')
	return builder.String(), nil
}
