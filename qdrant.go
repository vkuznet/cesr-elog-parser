package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	Collection string
	HTTP       *http.Client
}

func NewClient(url string, port int, collection string) *Client {
	return &Client{
		BaseURL:    fmt.Sprintf("%s:%d", url, port),
		Collection: collection,
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) UpsertDocs(docs []RAGDoc) error {
	points := make([]Point, 0, len(docs))

	for _, d := range docs {
		points = append(points, Point{
			ID:     d.ID,
			Vector: d.Vector,
			Payload: map[string]interface{}{
				"text":     d.Text,
				"mid":      d.Metadata.MID,
				"author":   d.Metadata.Author,
				"date":     d.Metadata.Date,
				"category": d.Metadata.Category,
				"system":   d.Metadata.System,
			},
		})
	}

	body := UpsertRequest{Points: points}

	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/collections/%s/points?wait=true",
		c.BaseURL, c.Collection)

	req, err := http.NewRequest("PUT", url, bytes.NewReader(raw))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("qdrant error: status=%d", resp.StatusCode)
	}

	return nil
}

func DummyEmbed(text string) []float32 {
	// replace with OpenAI, bge-small, etc.
	v := make([]float32, 384)
	for i := range text {
		v[i%384] += float32(text[i]) * 0.001
	}
	return v
}

type Point struct {
	ID      string                 `json:"id"`
	Vector  []float32              `json:"vector"`
	Payload map[string]interface{} `json:"payload"`
}

type UpsertRequest struct {
	Points []Point `json:"points"`
}

func inject2Qdrant(qdrantUrl, col, logDir string) error {
	p, err := ParseURLPort(qdrantUrl)
	if err != nil {
		return err
	}

	/*
		cfg := &qdrant.Config{Host: p.BaseURL, Port: p.Port}
		client, err := qdrant.NewClient(cfg)
		if err != nil {
			return err
		}
	*/

	client := NewClient(p.BaseURL, p.Port, col)

	entries := GetLogEntries(logDir, "ndjson")

	var docs []RAGDoc
	for _, e := range entries {
		docs = append(docs, ToRAGDoc(e))
	}

	if err := client.UpsertDocs(docs); err != nil {
		log.Fatal(err)
	}
	log.Println("ingested into qdrant")
	return nil
}

type ParsedEndpoint struct {
	BaseURL string
	Port    int
}

func ParseURLPort(input string) (*ParsedEndpoint, error) {
	// Ensure scheme exists so url.Parse works reliably
	if !strings.Contains(input, "://") {
		input = "http://" + input
	}

	u, err := url.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}

	host := u.Host

	// Split host and port
	h, p, err := net.SplitHostPort(host)
	if err != nil {
		return nil, fmt.Errorf("missing or invalid port in: %s", input)
	}

	port, err := strconv.Atoi(p)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	// Rebuild base URL without port
	baseURL := fmt.Sprintf("%s://%s", u.Scheme, h)

	return &ParsedEndpoint{
		BaseURL: baseURL,
		Port:    port,
	}, nil
}
