// Package docmeta strips AI-provenance metadata from the human-readable
// document formats: Markdown YAML frontmatter and HTML head metadata.
//
// This is separate from the Layer A rune scrub. A frontmatter key is visible
// text a human could delete by hand; the point of doing it here is that the
// key set and the vendor patterns are the same ones the rune scrub knows
// about, and both passes should agree on what counts as provenance.
package docmeta

import (
	"regexp"
	"strings"
)

// aiFrontmatterKeys are top-level YAML keys that carry generator provenance.
var aiFrontmatterKeys = map[string]bool{
	"generator": true, "ai": true, "ai_generated": true, "ai-generated": true,
	"claude": true, "anthropic": true, "openai": true, "gemini": true,
	"synthid": true, "c2pa": true, "content_credentials": true,
	"contentcredentials": true, "provenance": true, "digital_source_type": true,
	"digitalsourcetype": true, "created_with": true, "createdwith": true,
	"model": true, "llm": true,
}

var (
	aiNameRe = regexp.MustCompile(`(?i)generator|ai[-_ ]?generated|claude|anthropic|openai|gemini|synthid|c2pa|content.?credential|provenance|digital.?source|aigc`)
	// generatorAIRe separates an AI generator tag from ordinary CMS
	// provenance: <meta name="generator" content="WordPress"> is not a mark.
	generatorAIRe = regexp.MustCompile(`(?i)claude|anthropic|openai|chatgpt|gemini|synthid|copilot|midjourney|dall.?e|stable.?diffusion`)
	provenanceRe  = regexp.MustCompile(`(?i)DigitalSourceType|trainedAlgorithmicMedia|SoftwareAgent`)

	frontmatterRe = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---\r?\n?`)
	yamlKeyRe     = regexp.MustCompile(`^([A-Za-z0-9_.-]+)\s*:`)

	metaTagRe  = regexp.MustCompile(`(?i)<meta\b[^>]*>`)
	metaAttrRe = regexp.MustCompile(`(?i)(name|property|content|generator)\s*=\s*["']([^"']*)["']`)
	jsonLDRe   = regexp.MustCompile(`(?is)<script\b[^>]*type\s*=\s*["']application/ld\+json["'][^>]*>.*?</script>`)
	dataAIRe   = regexp.MustCompile(`(?i)\s\bdata-ai[\w-]*\s*=\s*["'][^"']*["']`)
	metaAIRe   = regexp.MustCompile(`(?i)generator|claude|anthropic|openai|gemini|synthid|c2pa|aigc`)
)

// Result is what an inspect or clean pass found in a document's metadata.
type Result struct {
	Format   string   `json:"format"`
	HasAI    bool     `json:"has_ai_metadata"`
	HasC2PA  bool     `json:"has_c2pa"`
	Findings []string `json:"findings"`
}

// Format identifies a document format from its filename extension, returning
// "" when the extension does not name one this package handles.
func Format(ext string) string {
	switch strings.ToLower(ext) {
	case ".md", ".markdown", ".mdx":
		return "markdown"
	case ".html", ".htm":
		return "html"
	}
	return ""
}

// Inspect reports the provenance metadata in text for the named format.
func Inspect(format, text string) Result {
	switch format {
	case "markdown":
		return inspectMarkdown(text)
	case "html":
		return inspectHTML(text)
	}
	return Result{Format: format}
}

// Clean removes that metadata, returning the new text and what it did.
func Clean(format, text string) (string, []string) {
	switch format {
	case "markdown":
		return cleanMarkdown(text)
	case "html":
		return cleanHTML(text)
	}
	return text, nil
}
