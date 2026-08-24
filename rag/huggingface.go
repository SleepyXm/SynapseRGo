package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultHuggingFaceEmbeddingEndpoint = "https://router.huggingface.co/hf-inference/models"
	maxEmbeddingResponseBytes           = 32 << 20
)

// HuggingFaceEmbedder uses Hugging Face's feature-extraction endpoint. Synapse
// chooses neither a default model nor a token; both must come from application
// configuration or the user's existing encrypted token store.
type HuggingFaceEmbedder struct {
	model    string
	token    string
	endpoint string
	client   *http.Client
}

func NewHuggingFaceEmbedder(model, token string) (*HuggingFaceEmbedder, error) {
	return newHuggingFaceEmbedder(model, token, defaultHuggingFaceEmbeddingEndpoint, &http.Client{
		Timeout: 90 * time.Second,
	})
}

func newHuggingFaceEmbedder(model, token, endpoint string, client *http.Client) (*HuggingFaceEmbedder, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, errors.New("Hugging Face embedding model is required")
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("Hugging Face token is required")
	}
	if strings.TrimSpace(endpoint) == "" {
		return nil, errors.New("Hugging Face endpoint is required")
	}
	if client == nil {
		return nil, errors.New("HTTP client is required")
	}

	return &HuggingFaceEmbedder{
		model:    model,
		token:    token,
		endpoint: strings.TrimRight(endpoint, "/"),
		client:   client,
	}, nil
}

func (e *HuggingFaceEmbedder) Model() string {
	return e.model
}

func (e *HuggingFaceEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	payload, err := json.Marshal(struct {
		Inputs    []string `json:"inputs"`
		Normalize bool     `json:"normalize"`
		Truncate  bool     `json:"truncate"`
	}{
		Inputs:    texts,
		Normalize: true,
		Truncate:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("encode embedding request: %w", err)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		e.endpoint+"/"+escapedModelPath(e.model),
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+e.token)
	request.Header.Set("Content-Type", "application/json")

	response, err := e.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request embeddings: %w", err)
	}
	defer response.Body.Close()

	limitedBody := &io.LimitedReader{R: response.Body, N: maxEmbeddingResponseBytes + 1}
	body, err := io.ReadAll(limitedBody)
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}
	if len(body) > maxEmbeddingResponseBytes {
		return nil, errors.New("embedding response exceeds size limit")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("embedding provider returned HTTP %d", response.StatusCode)
	}

	var vectors [][]float32
	if err := json.Unmarshal(body, &vectors); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	if len(vectors) != len(texts) {
		return nil, fmt.Errorf("embedding provider returned %d vectors for %d inputs", len(vectors), len(texts))
	}

	return vectors, nil
}

func escapedModelPath(model string) string {
	parts := strings.Split(strings.Trim(model, "/"), "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return strings.Join(parts, "/")
}
