package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// ElogEntry is one parsed record from an elog file.
// All fields are exported so they serialise cleanly to JSON / NDJSON.
type ElogEntry struct {
	// ---- identity ----
	MID        string `json:"mid"`         // $@MID@$: 69763
	SourceFile string `json:"source_file"` // originating file path

	// ---- header fields ----
	Date       string    `json:"date_raw"` // original header string
	ParsedDate time.Time `json:"date,omitempty"`
	Author     string    `json:"author"`
	Subject    string    `json:"subject"`
	Category   string    `json:"category"`
	System     string    `json:"system,omitempty"`
	Attention  string    `json:"attention,omitempty"`
	Attachment string    `json:"attachment,omitempty"`
	Encoding   string    `json:"encoding,omitempty"`

	// ---- body ----
	BodyRaw  string `json:"body_raw"`  // original body text
	BodyText string `json:"body_text"` // HTML-stripped plain text
	HasHTML  bool   `json:"has_html"`
	HasPlot  bool   `json:"has_plot"` // body references a plot file

	// ---- derived / for graph loading ----
	AuthorFirst string `json:"author_first,omitempty"`
	AuthorLast  string `json:"author_last,omitempty"`
}

// processFile reads one elog file, parses all entries inside it,
// and writes them as NDJSON (one JSON object per line) to outPath.
func processFile(inPath, outPath string) error {
	f, err := os.Open(inPath)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	entries, err := parseElogFile(f, inPath)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	if len(entries) == 0 {
		return nil // empty file – no output needed
	}

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer out.Close()

	enc := json.NewEncoder(out)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return fmt.Errorf("encode: %w", err)
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────
// Parser state machine
// ─────────────────────────────────────────────────────────

type parseState int

const (
	stateWaitREM parseState = iota // looking for "REM" sentinel
	stateHeader                    // reading header key: value lines
	stateBody                      // reading message body
)

const separator = "========================================"

// helper function to process elog file
func parseElogFile(r *os.File, sourcePath string) ([]ElogEntry, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	var (
		entries []ElogEntry
		current ElogEntry
		bodyBuf strings.Builder
		inBody  bool
	)

	flush := func() {
		if current.MID == "" {
			return
		}
		current.SourceFile = sourcePath

		raw := bodyBuf.String()
		current.BodyRaw = strings.TrimSpace(raw)
		current.BodyText = strings.TrimSpace(stripHTML(raw))
		current.HasHTML = isHTML(raw)
		current.HasPlot = hasPlotRef(raw)
		splitAuthor(&current)

		entries = append(entries, current)

		current = ElogEntry{}
		bodyBuf.Reset()
		inBody = false
	}

	for scanner.Scan() {
		line := scanner.Text()
		trim := strings.TrimSpace(line)

		// 🔑 NEW ENTRY DETECTED
		if isMIDLine(trim) {
			flush()
			parseHeaderLine(trim, &current)
			inBody = false
			continue
		}

		// separator → switch to body
		if strings.HasPrefix(line, separator) {
			inBody = true
			continue
		}

		if inBody {
			bodyBuf.WriteString(line)
			bodyBuf.WriteByte('\n')
			continue
		}

		// otherwise still header
		parseHeaderLine(line, &current)
	}

	// flush last entry
	flush()

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// ─────────────────────────────────────────────────────────
// Header parsing
// ─────────────────────────────────────────────────────────

func parseHeaderLine(line string, e *ElogEntry) {
	// $@MID@$: 69763  – special mid marker
	if strings.HasPrefix(line, "$@MID@$:") {
		e.MID = strings.TrimSpace(strings.TrimPrefix(line, "$@MID@$:"))
		return
	}

	key, val, ok := strings.Cut(line, ":")
	if !ok {
		return
	}
	key = strings.TrimSpace(key)
	val = strings.TrimSpace(val)

	switch strings.ToLower(key) {
	case "date":
		e.Date = val
		e.ParsedDate = parseDate(val)
	case "author":
		e.Author = val
	case "subject":
		e.Subject = val
	case "category":
		e.Category = val
	case "system":
		e.System = val
	case "attention list", "attentions":
		if val != "" {
			e.Attention = val
		}
	case "attachment":
		e.Attachment = val
	case "encoding":
		e.Encoding = val
	}
}

// parseDate tries a few common RFC formats used in elog headers.
func parseDate(s string) time.Time {
	formats := []string{
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"2 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 -0700",
		time.RFC1123Z,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// splitAuthor splits "FirstName LastName" into separate fields.
func splitAuthor(e *ElogEntry) {
	parts := strings.Fields(e.Author)
	if len(parts) >= 2 {
		e.AuthorFirst = parts[0]
		e.AuthorLast = strings.Join(parts[1:], " ")
	} else if len(parts) == 1 {
		e.AuthorFirst = parts[0]
	}
}

func isMIDLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "$@MID@$:")
}
