package rag

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestHuggingFaceEmbedderUsesFeatureExtractionContract(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path != "/example/embedding-model" {
				t.Errorf("request path = %q, want /example/embedding-model", request.URL.Path)
			}
			if got := request.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Errorf("authorization = %q, want bearer token", got)
			}

			var payload struct {
				Inputs    []string `json:"inputs"`
				Normalize bool     `json:"normalize"`
				Truncate  bool     `json:"truncate"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			if len(payload.Inputs) != 2 || !payload.Normalize || !payload.Truncate {
				t.Errorf("unexpected feature-extraction payload: %+v", payload)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`[[0.1,0.2],[0.3,0.4]]`)),
			}, nil
		}),
	}

	embedder, err := newHuggingFaceEmbedder(
		"example/embedding-model",
		"test-token",
		"https://embedding.test",
		client,
	)
	if err != nil {
		t.Fatal(err)
	}

	vectors, err := embedder.Embed(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 2 || len(vectors[0]) != 2 || vectors[1][1] != float32(0.4) {
		t.Fatalf("unexpected vectors: %+v", vectors)
	}
}
