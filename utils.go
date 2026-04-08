package main

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ParsedEndpoint struct {
	BaseURL string
	Port    int
}

func ParseURLPort(input string) (*ParsedEndpoint, error) {
	if !strings.Contains(input, "://") {
		input = "http://" + input
	}

	u, err := url.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}

	h, p, err := net.SplitHostPort(u.Host)
	if err != nil {
		return nil, fmt.Errorf("missing or invalid port in: %s", input)
	}

	port, err := strconv.Atoi(p)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	baseURL := fmt.Sprintf("%s://%s", u.Scheme, h)
	return &ParsedEndpoint{BaseURL: baseURL, Port: port}, nil
}

// ToRAGDoc converts a parsed ElogEntry into a RAGDoc ready for embedding and
// injection into Qdrant.
//
// Key corrections vs the previous version:
//  1. ID is now deterministic (deterministicID(MID)) instead of uuid.New(),
//     so re-running the injector upserts rather than duplicates.
//  2. SourceFile is propagated into ElogMetadata so ToPoint() can populate
//     the "source", "filename", and "relative_path" payload fields the
//     chatbot expects.
//  3. buildRAGText produces a metadata-prefixed plain-text chunk rather than
//     a prose sentence, which embeds better for semantic search.
func ToRAGDoc(e ElogEntry, dim int) RAGDoc {
	text := buildRAGText(e)

	return RAGDoc{
		ID:         deterministicID(e.MID),
		Text:       text,
		ChunkIndex: 0,
		Vector:     DummyEmbed(text, dim),
		Metadata: ElogMetadata{
			MID:        e.MID,
			Author:     e.Author,
			Date:       e.ParsedDate.Format(time.RFC3339),
			Category:   e.Category,
			System:     e.System,
			Subject:    e.Subject,
			SourceFile: e.SourceFile, // feeds "source" / "filename" in payload
			HasHTML:    e.HasHTML,
			HasPlot:    e.HasPlot,
		},
	}
}

// buildRAGText constructs the text that will be embedded into a vector.
//
// Format: structured metadata header followed by the stripped body text.
// Including author, date, category and subject in the embedded text means
// queries like "what did Jane report about the RF system in October 2021"
// will match even when those terms don't appear verbatim in the body.
func buildRAGText(e ElogEntry) string {
	date := e.ParsedDate.Format("2006-01-02 15:04")

	parts := []string{
		fmt.Sprintf("[Author: %s]", e.Author),
		fmt.Sprintf("[Date: %s]", date),
	}
	if e.Category != "" {
		parts = append(parts, fmt.Sprintf("[Category: %s]", e.Category))
	}
	if e.System != "" {
		parts = append(parts, fmt.Sprintf("[System: %s]", e.System))
	}
	if e.Subject != "" {
		parts = append(parts, fmt.Sprintf("[Subject: %s]", e.Subject))
	}

	header := strings.Join(parts, " ")
	return header + "\n" + e.BodyText
}
