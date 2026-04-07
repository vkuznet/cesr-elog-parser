package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ElogMetadata carries the structured fields from a parsed elog record.
// These become searchable payload fields in Qdrant alongside the vector.
type ElogMetadata struct {
	MID      string
	Author   string
	Date     string // RFC3339
	Category string
	System   string
	Subject  string
	HasHTML  bool
	HasPlot  bool
}

// RAGDoc is the unit of data flowing from the elog pipeline into Qdrant.
// Text is the embeddable content; Vector is its dense representation.
type RAGDoc struct {
	// ID is a deterministic UUID-like string derived from MID so re-ingestion
	// is idempotent (Qdrant upserts by ID).
	ID       string
	Text     string
	Vector   []float32
	Metadata ElogMetadata
}

// ElogEntryToRAGDoc converts a parsed elog record into a RAGDoc ready for
// embedding and injection.  embed is called with the document text and must
// return a vector whose length matches the collection dimension.
//
// The embeddable text intentionally includes metadata fields (author, subject,
// category) so that semantic search over those attributes works correctly even
// when a user's query doesn't use the exact field names.
func ElogEntryToRAGDoc(
	mid, author, date, category, system, subject, bodyText string,
	hasHTML, hasPlot bool,
	embed func(string) []float32,
) RAGDoc {
	// Build a rich text chunk: metadata header + body.
	// The header gives the embedding model authorial and temporal context.
	text := fmt.Sprintf(
		"[Author: %s] [Date: %s] [Category: %s] [System: %s] [Subject: %s]\n%s",
		author, date, category, system, subject, bodyText,
	)

	return RAGDoc{
		ID:     deterministicID(mid),
		Text:   text,
		Vector: embed(text),
		Metadata: ElogMetadata{
			MID:      mid,
			Author:   author,
			Date:     date,
			Category: category,
			System:   system,
			Subject:  subject,
			HasHTML:  hasHTML,
			HasPlot:  hasPlot,
		},
	}
}

// ToPoint converts a RAGDoc to the transport-agnostic Point type.
func (d RAGDoc) ToPoint() Point {
	return Point{
		ID:     d.ID,
		Vector: d.Vector,
		Payload: map[string]any{
			"text":     d.Text,
			"mid":      d.Metadata.MID,
			"author":   d.Metadata.Author,
			"date":     d.Metadata.Date,
			"category": d.Metadata.Category,
			"system":   d.Metadata.System,
			"subject":  d.Metadata.Subject,
			"has_html": d.Metadata.HasHTML,
			"has_plot": d.Metadata.HasPlot,
		},
	}
}

// DummyEmbed is a placeholder that produces a deterministic non-zero vector.
// Replace with a real embedding call (OpenAI, BGE, sentence-transformers, etc.)
// before using in production.
//
// The length is always exactly `dim` regardless of text length.
/*
func DummyEmbed(dim int) func(string) []float32 {
	return func(text string) []float32 {
		v := make([]float32, dim)
		for i, ch := range text {
			v[i%dim] += float32(ch) * 0.001
		}
		return v
	}
}
*/

func DummyEmbed(text string) []float32 {
	// replace with OpenAI, bge-small, etc.
	v := make([]float32, 384)
	for i := range text {
		v[i%384] += float32(text[i]) * 0.001
	}
	return v
}

// deterministicID derives a stable Qdrant-compatible point ID from a MID.
// Qdrant requires IDs to be UUIDs or unsigned integers; we produce a
// UUID-shaped hex string from a SHA-256 prefix so it survives re-ingestion.
func deterministicID(mid string) string {
	h := sha256.Sum256([]byte("elog:" + mid))
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

func GetLogEntries(logDir string, ext string) []ElogEntry {
	var entries []ElogEntry

	filepath.WalkDir(logDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip broken paths
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(d.Name(), ext) {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			var e ElogEntry
			if err := json.Unmarshal([]byte(line), &e); err != nil {
				// skip bad NDJSON line but continue processing
				continue
			}

			entries = append(entries, e)
		}

		return nil
	})

	return entries
}
