package docmeta

import (
	"fmt"
	"strings"
)

// isContinuation reports whether a frontmatter line belongs to the block
// opened by the previous top-level key (nested mapping or list item).
func isContinuation(line string) bool {
	return len(line) > 0 && (line[0] == ' ' || line[0] == '\t' || line[0] == '-')
}

func inspectMarkdown(text string) Result {
	res := Result{Format: "markdown"}
	m := frontmatterRe.FindStringSubmatch(text)
	if m == nil {
		return res
	}
	for _, line := range strings.Split(m[1], "\n") {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || isContinuation(line) {
			continue
		}
		km := yamlKeyRe.FindStringSubmatch(line)
		if km == nil {
			continue
		}
		key := km[1]
		_, value, _ := strings.Cut(line, ":")
		switch {
		case aiFrontmatterKeys[strings.ToLower(key)] || aiNameRe.MatchString(key):
			res.HasAI = true
			res.Findings = append(res.Findings, "frontmatter key: "+key)
		case aiNameRe.MatchString(value):
			res.HasAI = true
			res.Findings = append(res.Findings, "frontmatter value hit on "+key)
		}
	}
	for _, f := range res.Findings {
		if strings.Contains(strings.ToLower(f), "c2pa") {
			res.HasC2PA = true
		}
	}
	return res
}

func cleanMarkdown(text string) (string, []string) {
	m := frontmatterRe.FindStringSubmatchIndex(text)
	if m == nil {
		return text, []string{"no YAML frontmatter"}
	}
	block := text[m[2]:m[3]]
	body := text[m[1]:]

	var actions []string
	var kept []string
	dropping := false // inside the nested block of a dropped top-level key

	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)

		// Blank lines, comments and continuations belong to whichever block
		// they sit inside, so they follow its fate.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || isContinuation(line) {
			if !dropping {
				kept = append(kept, line)
			}
			continue
		}

		km := yamlKeyRe.FindStringSubmatch(line)
		if km == nil {
			dropping = false
			kept = append(kept, line)
			continue
		}

		key := km[1]
		_, value, _ := strings.Cut(line, ":")
		switch {
		case aiFrontmatterKeys[strings.ToLower(key)] || aiNameRe.MatchString(key):
			actions = append(actions, "drop frontmatter key: "+key)
			dropping = true
		case aiNameRe.MatchString(value):
			actions = append(actions, fmt.Sprintf("drop frontmatter key (value hit): %s", key))
			dropping = true
		default:
			dropping = false
			kept = append(kept, line)
		}
	}

	if actions == nil {
		actions = []string{"no AI frontmatter keys removed"}
	}
	newBlock := strings.Trim(strings.Join(kept, "\n"), "\n")
	if newBlock == "" {
		actions = append(actions, "removed empty frontmatter block")
		return strings.TrimLeft(body, "\n"), actions
	}
	return "---\n" + newBlock + "\n---\n" + body, actions
}
