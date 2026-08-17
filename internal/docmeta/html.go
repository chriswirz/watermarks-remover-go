package docmeta

import (
	"fmt"
	"strings"
)

func metaAttrs(tag string) map[string]string {
	attrs := map[string]string{}
	for _, m := range metaAttrRe.FindAllStringSubmatch(tag, -1) {
		attrs[strings.ToLower(m[1])] = m[2]
	}
	return attrs
}

// isCMSGenerator reports whether a generator meta tag is ordinary CMS
// provenance rather than an AI mark. WordPress naming its own version is not
// something this tool should silently delete from someone's page.
func isCMSGenerator(tag string) bool {
	attrs := metaAttrs(tag)
	nameOrProp := attrs["name"]
	if nameOrProp == "" {
		nameOrProp = attrs["property"]
	}
	if nameOrProp == "" {
		nameOrProp = attrs["generator"]
	}
	if !strings.EqualFold(nameOrProp, "generator") {
		return false
	}
	return !generatorAIRe.MatchString(attrs["content"]) && !generatorAIRe.MatchString(tag)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func inspectHTML(text string) Result {
	res := Result{Format: "html"}

	for _, tag := range metaTagRe.FindAllString(text, -1) {
		if strings.Contains(strings.ToLower(tag), "c2pa") ||
			strings.Contains(strings.ToLower(tag), "content-credential") ||
			strings.Contains(strings.ToLower(tag), "contentcredential") {
			res.HasC2PA = true
		}
		if isCMSGenerator(tag) {
			res.Findings = append(res.Findings, "info: cms generator: "+truncate(tag, 120))
			continue
		}
		if aiNameRe.MatchString(tag) {
			res.HasAI = true
			res.Findings = append(res.Findings, "meta: "+truncate(tag, 120))
		}
	}

	for _, blob := range jsonLDRe.FindAllString(text, -1) {
		if aiNameRe.MatchString(blob) || provenanceRe.MatchString(blob) {
			res.HasAI = true
			res.Findings = append(res.Findings, "json-ld provenance-like block")
			if strings.Contains(strings.ToLower(blob), "c2pa") ||
				strings.Contains(strings.ToLower(blob), "contentcredential") {
				res.HasC2PA = true
			}
		}
	}

	for _, attr := range dataAIRe.FindAllString(text, -1) {
		res.HasAI = true
		res.Findings = append(res.Findings, "attr: "+truncate(strings.TrimSpace(attr), 80))
	}
	return res
}

func cleanHTML(text string) (string, []string) {
	var actions []string

	out := metaTagRe.ReplaceAllStringFunc(text, func(tag string) string {
		if isCMSGenerator(tag) {
			return tag
		}
		if aiNameRe.MatchString(tag) || metaAIRe.MatchString(tag) {
			actions = append(actions, "drop meta: "+truncate(tag, 80))
			return ""
		}
		return tag
	})

	out = jsonLDRe.ReplaceAllStringFunc(out, func(blob string) string {
		if aiNameRe.MatchString(blob) || provenanceRe.MatchString(blob) {
			actions = append(actions, "drop json-ld provenance-like script")
			return ""
		}
		return blob
	})

	if n := len(dataAIRe.FindAllString(out, -1)); n > 0 {
		out = dataAIRe.ReplaceAllString(out, "")
		actions = append(actions, fmt.Sprintf("drop data-ai* attributes x%d", n))
	}

	if actions == nil {
		actions = []string{"no HTML AI meta removed"}
	}
	return out, actions
}
