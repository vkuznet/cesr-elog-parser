package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Metadata struct {
	MID      string `json:"mid"`
	Author   string `json:"author"`
	Date     string `json:"date"`
	Category string `json:"category"`
	System   string `json:"system"`
}

type RAGDoc struct {
	ID       string
	Text     string
	Vector   []float32
	Metadata Metadata
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

func ToRAGDoc(e ElogEntry) RAGDoc {
	text := buildRAGText(e)

	return RAGDoc{
		ID:     e.MID,
		Text:   text,
		Vector: DummyEmbed(text),
		Metadata: Metadata{
			MID:      e.MID,
			Author:   e.Author,
			Date:     e.ParsedDate.Format(time.RFC3339),
			Category: e.Category,
			System:   e.System,
		},
	}
}

func chunkText(text string, maxLen int) []string {
	var chunks []string
	runes := []rune(text)

	for i := 0; i < len(runes); i += maxLen {
		end := i + maxLen
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
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
