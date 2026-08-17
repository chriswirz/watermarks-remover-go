package docmeta

import (
	"strings"
	"testing"
)

func TestMarkdownDropsAIFrontmatter(t *testing.T) {
	in := "---\ntitle: My Post\ngenerator: Claude\nauthor: Ada\n---\n\nBody text.\n"
	out, actions := cleanMarkdown(in)
	if strings.Contains(out, "generator") {
		t.Errorf("generator key survived:\n%s", out)
	}
	for _, want := range []string{"title: My Post", "author: Ada", "Body text."} {
		if !strings.Contains(out, want) {
			t.Errorf("lost %q from output:\n%s", want, out)
		}
	}
	if len(actions) != 1 || !strings.Contains(actions[0], "generator") {
		t.Errorf("actions = %v", actions)
	}
}

func TestMarkdownDropsNestedBlockOfDroppedKey(t *testing.T) {
	in := "---\ntitle: Post\nprovenance:\n  model: claude\n  version: 3\ntags:\n  - go\n---\nBody\n"
	out, _ := cleanMarkdown(in)
	if strings.Contains(out, "model: claude") || strings.Contains(out, "version: 3") {
		t.Errorf("nested block of a dropped key survived:\n%s", out)
	}
	if !strings.Contains(out, "- go") {
		t.Errorf("unrelated nested list dropped:\n%s", out)
	}
}

func TestMarkdownRemovesEmptyFrontmatterBlock(t *testing.T) {
	out, actions := cleanMarkdown("---\ngenerator: Claude\n---\nBody\n")
	if out != "Body\n" {
		t.Errorf("got %q, want %q", out, "Body\n")
	}
	if !strings.Contains(strings.Join(actions, " "), "removed empty frontmatter") {
		t.Errorf("actions = %v", actions)
	}
}

func TestMarkdownWithoutFrontmatterIsUntouched(t *testing.T) {
	in := "# Title\n\nSome prose with a --- rule.\n"
	if out, _ := cleanMarkdown(in); out != in {
		t.Errorf("got %q, want unchanged", out)
	}
}

func TestMarkdownValueHit(t *testing.T) {
	res := inspectMarkdown("---\nnotes: written by Claude\n---\nBody\n")
	if !res.HasAI {
		t.Fatal("HasAI = false for a vendor name in a value")
	}
	if len(res.Findings) != 1 || !strings.Contains(res.Findings[0], "value hit") {
		t.Errorf("findings = %v", res.Findings)
	}
}

func TestHTMLDropsAIMetaKeepsCMS(t *testing.T) {
	in := `<head>` +
		`<meta name="generator" content="WordPress 6.4">` +
		`<meta name="generator" content="Claude">` +
		`<meta name="description" content="A page">` +
		`</head>`
	out, actions := cleanHTML(in)
	if !strings.Contains(out, "WordPress 6.4") {
		t.Errorf("CMS generator was dropped:\n%s", out)
	}
	if strings.Contains(out, "Claude") {
		t.Errorf("AI generator survived:\n%s", out)
	}
	if !strings.Contains(out, `name="description"`) {
		t.Errorf("unrelated meta dropped:\n%s", out)
	}
	if len(actions) != 1 {
		t.Errorf("actions = %v, want exactly one drop", actions)
	}
}

func TestHTMLDropsJSONLDProvenance(t *testing.T) {
	in := `<script type="application/ld+json">{"digitalSourceType":"trainedAlgorithmicMedia"}</script><p>hi</p>`
	out, _ := cleanHTML(in)
	if strings.Contains(out, "trainedAlgorithmicMedia") {
		t.Errorf("provenance block survived:\n%s", out)
	}
	if !strings.Contains(out, "<p>hi</p>") {
		t.Errorf("body dropped:\n%s", out)
	}
}

func TestHTMLKeepsUnrelatedJSONLD(t *testing.T) {
	in := `<script type="application/ld+json">{"@type":"Recipe","name":"Soup"}</script>`
	if out, _ := cleanHTML(in); out != in {
		t.Errorf("unrelated JSON-LD dropped:\n%s", out)
	}
}

func TestHTMLDropsDataAIAttributes(t *testing.T) {
	out, actions := cleanHTML(`<div data-ai-model="claude" class="x">hi</div>`)
	if strings.Contains(out, "data-ai-model") {
		t.Errorf("attribute survived:\n%s", out)
	}
	if !strings.Contains(out, `class="x"`) {
		t.Errorf("unrelated attribute dropped:\n%s", out)
	}
	if !strings.Contains(strings.Join(actions, " "), "data-ai*") {
		t.Errorf("actions = %v", actions)
	}
}

func TestInspectHTMLFlagsCMSAsInfoOnly(t *testing.T) {
	res := inspectHTML(`<meta name="generator" content="Hugo 0.120">`)
	if res.HasAI {
		t.Error("HasAI = true for a plain CMS generator")
	}
	if len(res.Findings) != 1 || !strings.HasPrefix(res.Findings[0], "info:") {
		t.Errorf("findings = %v", res.Findings)
	}
}

func TestFormat(t *testing.T) {
	for ext, want := range map[string]string{
		".md": "markdown", ".MD": "markdown", ".mdx": "markdown",
		".html": "html", ".htm": "html", ".txt": "", "": "",
	} {
		if got := Format(ext); got != want {
			t.Errorf("Format(%q) = %q, want %q", ext, got, want)
		}
	}
}

func TestCleanIsIdempotent(t *testing.T) {
	for _, tc := range []struct{ format, in string }{
		{"markdown", "---\ntitle: T\ngenerator: Claude\n---\nBody\n"},
		{"html", `<meta name="generator" content="Claude"><p data-ai-x="1">hi</p>`},
	} {
		once, _ := Clean(tc.format, tc.in)
		if twice, _ := Clean(tc.format, once); twice != once {
			t.Errorf("%s: second clean changed the text:\n%q\n%q", tc.format, once, twice)
		}
	}
}
