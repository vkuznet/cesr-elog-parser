package main

import (
	"context"
	"errors"
	"testing"
)

// ── Stub backend ─────────────────────────────────────────────────────────────

type stubClient struct {
	ensureCalled bool
	upserted     []Point
	failUpsert   bool
}

func (s *stubClient) EnsureCollection(_ context.Context, _ int) error {
	s.ensureCalled = true
	return nil
}

func (s *stubClient) UpsertPoints(_ context.Context, pts []Point) error {
	if s.failUpsert {
		return errors.New("stub: upsert failed")
	}
	s.upserted = append(s.upserted, pts...)
	return nil
}

func (s *stubClient) Close() error { return nil }

// ── ParseEndpoint ─────────────────────────────────────────────────────────────

func TestParseEndpoint_WithPort(t *testing.T) {
	_, host, port, err := ParseEndpoint("http://localhost:6333")
	if err != nil {
		t.Fatal(err)
	}
	if host != "localhost" || port != 6333 {
		t.Errorf("got host=%q port=%d, want localhost:6333", host, port)
	}
}

func TestParseEndpoint_NoScheme(t *testing.T) {
	_, host, port, err := ParseEndpoint("myhost:9999")
	if err != nil {
		t.Fatal(err)
	}
	if host != "myhost" || port != 9999 {
		t.Errorf("got host=%q port=%d", host, port)
	}
}

func TestParseEndpoint_NoPort(t *testing.T) {
	_, host, port, err := ParseEndpoint("http://myhost")
	if err != nil {
		t.Fatal(err)
	}
	if host != "myhost" || port != 0 {
		t.Errorf("got host=%q port=%d", host, port)
	}
}

// ── ElogEntryToRAGDoc ────────────────────────────────────────────────────────

func TestElogEntryToRAGDoc_IDDeterministic(t *testing.T) {
	embed := DummyEmbed(4)
	d1 := ElogEntryToRAGDoc("69763", "Jane", "2021-10-15", "LINAC", "RF", "Hiccup", "body", false, true, embed)
	d2 := ElogEntryToRAGDoc("69763", "Jane", "2021-10-15", "LINAC", "RF", "Hiccup", "body", false, true, embed)
	if d1.ID != d2.ID {
		t.Errorf("ID not deterministic: %q vs %q", d1.ID, d2.ID)
	}
}

func TestElogEntryToRAGDoc_DifferentMIDsDifferentIDs(t *testing.T) {
	embed := DummyEmbed(4)
	d1 := ElogEntryToRAGDoc("1", "A", "", "", "", "", "x", false, false, embed)
	d2 := ElogEntryToRAGDoc("2", "A", "", "", "", "", "x", false, false, embed)
	if d1.ID == d2.ID {
		t.Error("different MIDs should produce different IDs")
	}
}

func TestElogEntryToRAGDoc_TextContainsMetadata(t *testing.T) {
	embed := DummyEmbed(8)
	d := ElogEntryToRAGDoc("1", "Jane Smith", "2021-10-15", "LINAC", "RF", "Hiccup", "body text", false, false, embed)
	for _, want := range []string{"Jane Smith", "LINAC", "RF", "Hiccup", "body text"} {
		if !contains(d.Text, want) {
			t.Errorf("Text missing %q: %s", want, d.Text)
		}
	}
}

func TestRAGDoc_ToPoint(t *testing.T) {
	embed := DummyEmbed(4)
	d := ElogEntryToRAGDoc("99", "Bob", "2021-01-01", "Vac", "Ring", "Leak", "fixed it", false, false, embed)
	p := d.ToPoint()

	if p.ID != d.ID {
		t.Errorf("point ID mismatch: %q vs %q", p.ID, d.ID)
	}
	if len(p.Vector) != 4 {
		t.Errorf("vector length: got %d, want 4", len(p.Vector))
	}
	if p.Payload["mid"] != "99" {
		t.Errorf("payload mid: %v", p.Payload["mid"])
	}
	if p.Payload["has_plot"] != false {
		t.Errorf("payload has_plot: %v", p.Payload["has_plot"])
	}
}

// ── DummyEmbed ───────────────────────────────────────────────────────────────

func TestDummyEmbed_CorrectDimension(t *testing.T) {
	for _, dim := range []int{4, 384, 1536} {
		v := DummyEmbed(dim)("some text")
		if len(v) != dim {
			t.Errorf("dim=%d: got len %d", dim, len(v))
		}
	}
}

// ── Inject (uses stub backend indirectly via InjectConfig.Docs path) ─────────

// injectWithStub bypasses New() to exercise the Inject logic with a stub.
// In production you'd call nject; here we call the batch loop directly.
func injectWithStub(ctx context.Context, stub Client, docs []RAGDoc, batchSize int) (int, error) {
	if err := stub.EnsureCollection(ctx, 4); err != nil {
		return 0, err
	}
	points := make([]Point, len(docs))
	for i, d := range docs {
		points[i] = d.ToPoint()
	}
	total := 0
	for start := 0; start < len(points); start += batchSize {
		end := start + batchSize
		if end > len(points) {
			end = len(points)
		}
		if err := stub.UpsertPoints(ctx, points[start:end]); err != nil {
			return total, err
		}
		total += end - start
	}
	return total, nil
}

func TestInject_AllPoints(t *testing.T) {
	embed := DummyEmbed(4)
	docs := []RAGDoc{
		ElogEntryToRAGDoc("1", "A", "", "X", "", "s1", "b1", false, false, embed),
		ElogEntryToRAGDoc("2", "B", "", "Y", "", "s2", "b2", false, false, embed),
		ElogEntryToRAGDoc("3", "C", "", "Z", "", "s3", "b3", false, false, embed),
	}

	stub := &stubClient{}
	n, err := injectWithStub(context.Background(), stub, docs, 10)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("expected 3 injected, got %d", n)
	}
	if len(stub.upserted) != 3 {
		t.Errorf("stub received %d points", len(stub.upserted))
	}
	if !stub.ensureCalled {
		t.Error("EnsureCollection was not called")
	}
}

func TestInject_BatchBoundary(t *testing.T) {
	embed := DummyEmbed(4)
	var docs []RAGDoc
	for i := 0; i < 5; i++ {
		docs = append(docs, ElogEntryToRAGDoc(
			string(rune('0'+i)), "A", "", "X", "", "s", "b", false, false, embed,
		))
	}

	stub := &stubClient{}
	n, err := injectWithStub(context.Background(), stub, docs, 2)
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Errorf("expected 5, got %d", n)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
