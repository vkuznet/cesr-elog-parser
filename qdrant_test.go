package main

import (
	"strings"
	"testing"
	"time"
)

// ── ToRAGDoc / buildRAGText ───────────────────────────────────────────────────

func sampleEntry() ElogEntry {
	return ElogEntry{
		MID:        "69763",
		SourceFile: "/data/logs/2021-10-15.log",
		Author:     "Jane Smith",
		AuthorFirst: "Jane",
		AuthorLast:  "Smith",
		Subject:    "Linac Forward Power Hiccup",
		Category:   "LINAC",
		System:     "RF",
		BodyText:   "When about to inject, the beam degraded.",
		HasHTML:    false,
		HasPlot:    true,
		ParsedDate: time.Date(2021, 10, 15, 11, 42, 53, 0, time.UTC),
	}
}

func TestToRAGDoc_IDIsDeterministic(t *testing.T) {
	e := sampleEntry()
	d1 := ToRAGDoc(e, 4)
	d2 := ToRAGDoc(e, 4)
	if d1.ID != d2.ID {
		t.Errorf("ID not deterministic: %q vs %q", d1.ID, d2.ID)
	}
}

func TestToRAGDoc_DifferentMIDsDifferentIDs(t *testing.T) {
	e1, e2 := sampleEntry(), sampleEntry()
	e2.MID = "99999"
	if ToRAGDoc(e1, 4).ID == ToRAGDoc(e2, 4).ID {
		t.Error("different MIDs must produce different IDs")
	}
}

func TestToRAGDoc_SourceFilePropagated(t *testing.T) {
	e := sampleEntry()
	d := ToRAGDoc(e, 4)
	if d.Metadata.SourceFile != e.SourceFile {
		t.Errorf("SourceFile not propagated: got %q", d.Metadata.SourceFile)
	}
}

func TestBuildRAGText_ContainsMetadataAndBody(t *testing.T) {
	e := sampleEntry()
	text := buildRAGText(e)
	for _, want := range []string{
		"Jane Smith", "LINAC", "RF", "Linac Forward Power Hiccup",
		"2021-10-15", "When about to inject",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("buildRAGText missing %q in:\n%s", want, text)
		}
	}
}

func TestBuildRAGText_OptionalFieldsOmitted(t *testing.T) {
	e := sampleEntry()
	e.System = ""
	e.Category = ""
	text := buildRAGText(e)
	if strings.Contains(text, "[System:") || strings.Contains(text, "[Category:") {
		t.Errorf("empty fields should not appear in text: %s", text)
	}
}

// ── ToPoint payload schema alignment ─────────────────────────────────────────

func TestToPoint_RequiredChatbotFields(t *testing.T) {
	e := sampleEntry()
	p := ToRAGDoc(e, 4).ToPoint()

	required := []string{"type", "source", "relative_path", "filename", "chunk_index", "text"}
	for _, field := range required {
		if _, ok := p.Payload[field]; !ok {
			t.Errorf("payload missing required chatbot field %q", field)
		}
	}
}

func TestToPoint_TypeIsDocument(t *testing.T) {
	p := ToRAGDoc(sampleEntry(), 4).ToPoint()
	if p.Payload["type"] != "document" {
		t.Errorf("type: got %v, want \"document\"", p.Payload["type"])
	}
}

func TestToPoint_SourceIsAbsolutePath(t *testing.T) {
	p := ToRAGDoc(sampleEntry(), 4).ToPoint()
	if p.Payload["source"] != "/data/logs/2021-10-15.log" {
		t.Errorf("source: got %v", p.Payload["source"])
	}
}

func TestToPoint_FilenameIsBasename(t *testing.T) {
	p := ToRAGDoc(sampleEntry(), 4).ToPoint()
	for _, field := range []string{"filename", "relative_path"} {
		if p.Payload[field] != "2021-10-15.log" {
			t.Errorf("%s: got %v, want \"2021-10-15.log\"", field, p.Payload[field])
		}
	}
}

func TestToPoint_ChunkIndexIsZero(t *testing.T) {
	p := ToRAGDoc(sampleEntry(), 4).ToPoint()
	if p.Payload["chunk_index"] != 0 {
		t.Errorf("chunk_index: got %v, want 0", p.Payload["chunk_index"])
	}
}

func TestToPoint_TextMatchesBuildRAGText(t *testing.T) {
	e := sampleEntry()
	d := ToRAGDoc(e, 4)
	p := d.ToPoint()
	if p.Payload["text"] != d.Text {
		t.Error("payload text does not match RAGDoc.Text")
	}
}

func TestToPoint_ElogFieldsPresent(t *testing.T) {
	p := ToRAGDoc(sampleEntry(), 4).ToPoint()
	checks := map[string]any{
		"mid":      "69763",
		"author":   "Jane Smith",
		"category": "LINAC",
		"system":   "RF",
		"subject":  "Linac Forward Power Hiccup",
		"has_html": false,
		"has_plot": true,
	}
	for k, want := range checks {
		if p.Payload[k] != want {
			t.Errorf("payload[%q]: got %v, want %v", k, p.Payload[k], want)
		}
	}
}

// ── DummyEmbed ────────────────────────────────────────────────────────────────

func TestDummyEmbed_ExactDimension(t *testing.T) {
	for _, dim := range []int{4, 384, 1536} {
		v := DummyEmbed("hello world", dim)
		if len(v) != dim {
			t.Errorf("dim=%d: got len %d", dim, len(v))
		}
	}
}

// ── deterministicID ───────────────────────────────────────────────────────────

func TestDeterministicID_UUIDShape(t *testing.T) {
	id := deterministicID("69763")
	// UUID format: 8-4-4-4-12 hex chars with hyphens = 36 chars total
	if len(id) != 36 {
		t.Errorf("ID length: got %d, want 36: %q", len(id), id)
	}
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Errorf("ID not UUID-shaped: %q", id)
	}
}
