package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultHTTPPort    = 6333
	defaultHTTPTimeout = 30 * time.Second
)

// httpClient implements Client over Qdrant's REST API.
type httpClient struct {
	baseURL    string // e.g. "http://localhost:6333"  — no trailing slash
	collection string
	apiKey     string
	http       *http.Client
}

func newHTTPClient(cfg Config) (*httpClient, error) {
	_, host, port, err := ParseEndpoint(cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	if port == 0 {
		port = defaultHTTPPort
	}

	// Always use http:// for REST; TLS termination is handled by a proxy in
	// most self-hosted setups. Adjust here if you need direct TLS.
	baseURL := fmt.Sprintf("http://%s:%d", host, port)

	return &httpClient{
		baseURL:    baseURL,
		collection: cfg.Collection,
		apiKey:     cfg.APIKey,
		http: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}, nil
}

// ── Public interface methods ─────────────────────────────────────────────────

func (c *httpClient) EnsureCollection(ctx context.Context, vectorSize int) error {
	exists, err := c.collectionExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return c.createCollection(ctx, vectorSize)
}

func (c *httpClient) UpsertPoints(ctx context.Context, points []Point) error {
	type wirePoint struct {
		ID      string         `json:"id"`
		Vector  []float32      `json:"vector"`
		Payload map[string]any `json:"payload"`
	}
	type upsertRequest struct {
		Points []wirePoint `json:"points"`
	}

	wire := make([]wirePoint, len(points))
	for i, p := range points {
		wire[i] = wirePoint{ID: p.ID, Vector: p.Vector, Payload: p.Payload}
	}

	endpoint := fmt.Sprintf("%s/collections/%s/points?wait=true", c.baseURL, c.collection)
	return c.doJSON(ctx, http.MethodPut, endpoint, upsertRequest{Points: wire}, nil)
}

func (c *httpClient) Close() error { return nil } // HTTP client needs no teardown

// ── Internal helpers ─────────────────────────────────────────────────────────

func (c *httpClient) collectionExists(ctx context.Context) (bool, error) {
	endpoint := fmt.Sprintf("%s/collections/%s", c.baseURL, c.collection)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("qdrant http: check collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode >= 300 {
		return false, fmt.Errorf("qdrant http: check collection: status %d", resp.StatusCode)
	}
	return true, nil
}

func (c *httpClient) createCollection(ctx context.Context, vectorSize int) error {
	type vectorParams struct {
		Size     int    `json:"size"`
		Distance string `json:"distance"`
	}
	type createRequest struct {
		Vectors vectorParams `json:"vectors"`
	}

	body := createRequest{
		Vectors: vectorParams{Size: vectorSize, Distance: "Cosine"},
	}

	endpoint := fmt.Sprintf("%s/collections/%s", c.baseURL, c.collection)
	return c.doJSON(ctx, http.MethodPut, endpoint, body, nil)
}

// doJSON marshals reqBody, sends the request, and optionally decodes the
// response into respOut (pass nil to discard the body).
func (c *httpClient) doJSON(ctx context.Context, method, endpoint string, reqBody, respOut any) error {
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("qdrant http: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("qdrant http: build request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant http: %s %s: %w", method, endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant http: %s %s: status %d: %s", method, endpoint, resp.StatusCode, body)
	}

	if respOut != nil {
		if err := json.NewDecoder(resp.Body).Decode(respOut); err != nil {
			return fmt.Errorf("qdrant http: decode response: %w", err)
		}
	}
	return nil
}

func (c *httpClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("api-key", c.apiKey)
	}
}
