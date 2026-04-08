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
// These are stored as Qdrant payload fields alongside the embeddable text so
// the chatbot can filter, cite, and surface them in responses.
type ElogMetadata struct {
	MID        string
	Author     string
	Date       string // RFC3339
	Category   string
	System     string
	Subject    string
	SourceFile string // absolute path to the originating elog file
	HasHTML    bool
	HasPlot    bool
}

// RAGDoc is the unit of data flowing from the elog pipeline into Qdrant.
// Text is the embeddable content; Vector is its dense representation.
type RAGDoc struct {
	// ID is a deterministic UUID derived from MID so re-ingestion is idempotent.
	// Qdrant upserts by ID — a random uuid.New() would create duplicates on
	// every run instead of updating the existing point.
	ID         string
	Text       string
	ChunkIndex int // always 0 for full-entry records; >0 when body is split
	Vector     []float32
	Metadata   ElogMetadata
}

// ToPoint converts a RAGDoc into the transport-agnostic Point type.
//
// Payload field names are intentionally aligned with the chatbot's existing
// collection schema:
//
//	"type"          → always "document"  (chatbot filters on this)
//	"source"        → absolute path      (used for citations)
//	"relative_path" → filename only      (display name in UI)
//	"filename"      → filename only      (mirrors relative_path)
//	"chunk_index"   → integer            (position within the source entry)
//	"text"          → embeddable text    (what was embedded into the vector)
//
// Elog-specific fields (mid, author, date, …) are kept as extra payload so
// graph/filter queries still work without breaking the chatbot's expectations.
func (d RAGDoc) ToPoint() Point {
	filename := filepath.Base(d.Metadata.SourceFile)

	return Point{
		ID:     d.ID,
		Vector: d.Vector,
		Payload: map[string]any{
			// ── Fields the chatbot expects ──────────────────────────────
			"type":          "document",
			"source":        d.Metadata.SourceFile,
			"relative_path": filename,
			"filename":      filename,
			"chunk_index":   d.ChunkIndex,
			"text":          d.Text,

			// ── Elog-specific fields for filtering / graph queries ──────
			"mid":      d.Metadata.MID,
			"author":   d.Metadata.Author,
			"date":     d.Metadata.Date,
			"date_ts":  parseDate(d.Metadata.Date).Unix(),
			"category": d.Metadata.Category,
			"system":   d.Metadata.System,
			"subject":  d.Metadata.Subject,
			"has_html": d.Metadata.HasHTML,
			"has_plot": d.Metadata.HasPlot,
		},
	}
}

// DummyEmbed produces a deterministic non-zero vector of exactly dim floats.
// Replace with a real embedding call (OpenAI text-embedding-3-small,
// BGE-small-en, sentence-transformers, etc.) before using in production.
// The dim parameter must match the Qdrant collection's configured dimension.
func DummyEmbed(text string, dim int) []float32 {
	v := make([]float32, dim)
	for i, ch := range text {
		v[i%dim] += float32(ch) * 0.001
	}
	return v
}

// deterministicID derives a stable Qdrant-compatible UUID from a MID.
// Qdrant requires point IDs to be UUIDs or uint64s; we SHA-256 the MID and
// format the first 16 bytes as a UUID so the same entry always maps to the
// same ID across runs.
func deterministicID(mid string) string {
	h := sha256.Sum256([]byte("elog:" + mid))
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

// GetLogEntries reads all NDJSON files from logDir matching ext suffix
// and returns the parsed entries.
func GetLogEntries(logDir string, ext string) []ElogEntry {
	var entries []ElogEntry

	filepath.WalkDir(logDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
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
		scanner.Buffer(make([]byte, 2<<20), 2<<20)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var e ElogEntry
			if err := json.Unmarshal([]byte(line), &e); err != nil {
				continue
			}
			entries = append(entries, e)
		}
		return nil
	})

	return entries
}
