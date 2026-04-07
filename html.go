package main

import (
	"regexp"
	"strings"
)

var (
	// HTML tag pattern – handles attributes, self-closing, doctype.
	reHTMLTag = regexp.MustCompile(`(?i)<[^>]+>`)

	// Common plot/figure file extensions that can appear in elog bodies.
	rePlotRef = regexp.MustCompile(`(?i)\.(gif|png|jpg|jpeg|svg|pdf|eps|ps|tiff?)\b`)

	// Collapse runs of whitespace left behind after tag removal.
	reMultiSpace = regexp.MustCompile(`[ \t]{2,}`)
	reMultiNL    = regexp.MustCompile(`\n{3,}`)

	// HTML entities we commonly encounter.
	htmlEntities = strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&nbsp;", " ",
		"&ndash;", "–",
		"&mdash;", "—",
	)
)

// isHTML returns true if the body looks like it contains HTML markup.
func isHTML(s string) bool {
	return reHTMLTag.MatchString(s)
}

// hasPlotRef returns true if the body references an image or plot file.
func hasPlotRef(s string) bool {
	return rePlotRef.MatchString(s)
}

// stripHTML removes HTML tags, decodes common entities, and normalises whitespace.
// It does NOT invoke a full HTML parser intentionally – elog bodies are
// partial/messy HTML fragments where a lenient regex approach is more robust.
func stripHTML(s string) string {
	// 1. Remove script / style blocks entirely (content is not useful).
	s = removeBlock(s, "script")
	s = removeBlock(s, "style")

	// 2. Replace block-level elements with newlines to preserve paragraph breaks.
	s = replaceBlockTags(s)

	// 3. Strip remaining tags.
	s = reHTMLTag.ReplaceAllString(s, "")

	// 4. Decode entities.
	s = htmlEntities.Replace(s)

	// 5. Normalise whitespace.
	s = reMultiSpace.ReplaceAllString(s, " ")
	s = reMultiNL.ReplaceAllString(s, "\n\n")

	return strings.TrimSpace(s)
}

var reBlockTag = regexp.MustCompile(`(?i)<(br\s*/?|/?(p|div|h[1-6]|li|tr|td|th|blockquote|pre)[^>]*)>`)

// replaceBlockTags swaps common block-level/line-break tags for newlines.
func replaceBlockTags(s string) string {
	return reBlockTag.ReplaceAllString(s, "\n")
}

// removeBlock strips a paired HTML block element and all its contents.
func removeBlock(s, tag string) string {
	re := regexp.MustCompile(`(?is)<` + tag + `[^>]*>.*?</` + tag + `>`)
	return re.ReplaceAllString(s, "")
}
