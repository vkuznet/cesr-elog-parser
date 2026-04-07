package main

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

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

func ToRAGDoc(e ElogEntry) RAGDoc {
	text := buildRAGText(e)

	return RAGDoc{
		ID:     uuid.New().String(),
		Text:   text,
		Vector: DummyEmbed(text),
		Metadata: ElogMetadata{
			MID:      e.MID,
			Author:   e.Author,
			Date:     e.ParsedDate.Format(time.RFC3339),
			Category: e.Category,
			System:   e.System,
			Subject:  e.Subject,
			HasHTML:  e.HasHTML,
			HasPlot:  e.HasPlot,
		},
	}
}

func buildRAGText(e ElogEntry) string {
	date := e.ParsedDate.Format("January 2, 2006 15:04")

	header := fmt.Sprintf(
		"On %s, %s reported:",
		date,
		e.Author,
	)

	if e.Category != "" {
		header += " [" + e.Category + "]"
	}

	return header + "\n" + e.BodyText
}
