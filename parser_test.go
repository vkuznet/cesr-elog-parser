package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────
// Header parsing
// ─────────────────────────────────────────────────────────

func TestParseHeaderLine_MID(t *testing.T) {
	var e ElogEntry
	parseHeaderLine("$@MID@$: 69763", &e)
	if e.MID != "69763" {
		t.Errorf("MID: got %q, want %q", e.MID, "69763")
	}
}

func TestParseHeaderLine_Date(t *testing.T) {
	var e ElogEntry
	parseHeaderLine("Date: Fri, 15 Oct 2021 11:42:53 -0400", &e)
	if e.Date == "" {
		t.Fatal("Date not set")
	}
	want := time.Date(2021, 10, 15, 11, 42, 53, 0, time.FixedZone("", -4*3600))
	if !e.ParsedDate.Equal(want) {
		t.Errorf("ParsedDate: got %v, want %v", e.ParsedDate, want)
	}
}

func TestParseHeaderLine_Author(t *testing.T) {
	var e ElogEntry
	parseHeaderLine("Author: John Doe", &e)
	if e.Author != "John Doe" {
		t.Errorf("Author: got %q", e.Author)
	}
}

func TestSplitAuthor(t *testing.T) {
	cases := []struct{ in, first, last string }{
		{"John Doe", "John", "Doe"},
		{"Mary Jane Watson", "Mary", "Jane Watson"},
		{"Admin", "Admin", ""},
	}
	for _, c := range cases {
		e := ElogEntry{Author: c.in}
		splitAuthor(&e)
		if e.AuthorFirst != c.first || e.AuthorLast != c.last {
			t.Errorf("%q: got (%q, %q), want (%q, %q)",
				c.in, e.AuthorFirst, e.AuthorLast, c.first, c.last)
		}
	}
}

// ─────────────────────────────────────────────────────────
// HTML helpers
// ─────────────────────────────────────────────────────────

func TestIsHTML(t *testing.T) {
	if !isHTML("<p>Hello</p>") {
		t.Error("expected HTML detected")
	}
	if isHTML("Plain text log entry.") {
		t.Error("plain text should not be HTML")
	}
}

func TestHasPlotRef(t *testing.T) {
	if !hasPlotRef("See figure.gif attached") {
		t.Error("expected plot ref detected")
	}
	if hasPlotRef("No attachments here") {
		t.Error("no plot ref expected")
	}
}

func TestStripHTML(t *testing.T) {
	in := `<p>Hello <b>World</b></p><script>alert('x')</script><br/>Bye`
	out := stripHTML(in)
	if strings.Contains(out, "<") || strings.Contains(out, "alert") {
		t.Errorf("stripHTML left markup: %q", out)
	}
	if !strings.Contains(out, "Hello") || !strings.Contains(out, "World") {
		t.Errorf("stripHTML removed content: %q", out)
	}
}

// ─────────────────────────────────────────────────────────
// Full parse of a synthetic elog file
// ─────────────────────────────────────────────────────────

const sampleElog = `REM
$@MID@$: 69763
Date: Fri, 15 Oct 2021 11:42:53 -0400
Author: Jane Smith
Subject: Linac Forward Power Hiccup
Category: LINAC
System: RF
Attention List: Show
Attentions:
Attachment: 211015_114253_std_03050450.gif
Encoding: plain
========================================
When about to inject, I noticed the Synch beam quickly degrade to nothing.
See plot: result.png

REM
$@MID@$: 69764
Date: Sat, 16 Oct 2021 09:15:00 -0400
Author: Bob Builder
Subject: Vacuum leak fixed
Category: Vacuum
System: Ring
Encoding: plain
========================================
<p>Fixed the <b>vacuum</b> leak on sector 3.</p>
`

func TestParseElogFile_MultiEntry(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sample.log")
	if err := os.WriteFile(path, []byte(sampleElog), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	entries, err := parseElogFile(f, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	e := entries[0]
	if e.MID != "69763" {
		t.Errorf("MID: got %q", e.MID)
	}
	if e.Author != "Jane Smith" {
		t.Errorf("Author: got %q", e.Author)
	}
	if e.Category != "LINAC" {
		t.Errorf("Category: got %q", e.Category)
	}
	if !e.HasPlot {
		t.Error("expected HasPlot=true")
	}
	if e.HasHTML {
		t.Error("expected HasHTML=false for plain entry")
	}

	e2 := entries[1]
	if e2.MID != "69764" {
		t.Errorf("entry2 MID: got %q", e2.MID)
	}
	if !e2.HasHTML {
		t.Error("expected HasHTML=true for HTML entry")
	}
	if strings.Contains(e2.BodyText, "<p>") {
		t.Errorf("HTML not stripped from BodyText: %q", e2.BodyText)
	}
}

// ─────────────────────────────────────────────────────────
// processFile end-to-end
// ─────────────────────────────────────────────────────────

func TestProcessFile(t *testing.T) {
	tmp := t.TempDir()
	in := filepath.Join(tmp, "sample.log")
	out := filepath.Join(tmp, "sample.ndjson")

	if err := os.WriteFile(in, []byte(sampleElog), 0644); err != nil {
		t.Fatal(err)
	}
	if err := processFile(in, out); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 NDJSON lines, got %d", len(lines))
	}
}
