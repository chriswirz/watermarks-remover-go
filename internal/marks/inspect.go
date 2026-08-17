package marks

import "sort"

// Hit is one suspicious codepoint, aggregated across the whole input.
type Hit struct {
	Codepoint     string `json:"codepoint"`
	Label         string `json:"label"`
	Count         int    `json:"count"`
	Kind          Kind   `json:"kind"`
	Confidence    string `json:"confidence"`
	SampleOffsets []int  `json:"sample_offsets"`
}

// Report is the result of inspecting text for Layer A carriers.
type Report struct {
	Length          int      `json:"length"`
	SuspiciousTotal int      `json:"suspicious_total"`
	Hits            []Hit    `json:"hits"`
	Notes           []string `json:"notes"`
}

// InspectOptions selects how much the scan flags. Inspect always reports
// directional controls and exotic spaces; these widen it further.
type InspectOptions struct {
	// Aggressive also flags Latin confusable / fullwidth lookalikes.
	Aggressive bool
	// StripEmojiGlue also flags load-bearing invisibles (emoji glue, script
	// joiners, flag tags, same-script fillers, orthographic Cf).
	StripEmojiGlue bool
}

const maxSamples = 10

// Inspect reports the invisible Unicode and space homoglyphs in text without
// modifying it. Offsets are rune indices, not byte indices.
func Inspect(text string, opt InspectOptions) Report {
	rs := []rune(text)
	scanOpt := Options{
		AggressiveHomoglyphs: opt.Aggressive,
		StripEmojiGlue:       opt.StripEmojiGlue,
		StripBidi:            true, // inspect reports bidi even when clean keeps it
	}

	type bucket struct {
		r       rune
		kind    Kind
		offsets []int
	}
	type hitKey struct {
		r    rune
		kind Kind
	}
	buckets := map[hitKey]*bucket{}
	var order []hitKey

	scan(rs, scanOpt, func(i int, r rune, d decision) {
		if d.kind == "" {
			return
		}
		key := hitKey{r, d.kind}
		b, ok := buckets[key]
		if !ok {
			b = &bucket{r: r, kind: d.kind}
			buckets[key] = b
			order = append(order, key)
		}
		b.offsets = append(b.offsets, i)
	})

	sort.SliceStable(order, func(a, b int) bool {
		x, y := buckets[order[a]], buckets[order[b]]
		if len(x.offsets) != len(y.offsets) {
			return len(x.offsets) > len(y.offsets) // most frequent first
		}
		return x.r < y.r
	})

	hits := make([]Hit, 0, len(order))
	total := 0
	for _, key := range order {
		b := buckets[key]
		hits = append(hits, Hit{
			Codepoint:     sprintCodepoint(b.r),
			Label:         label(b.r),
			Count:         len(b.offsets),
			Kind:          b.kind,
			Confidence:    b.kind.Confidence(),
			SampleOffsets: b.offsets[:min(len(b.offsets), maxSamples)],
		})
		total += len(b.offsets)
	}

	return Report{
		Length:          len(rs),
		SuspiciousTotal: total,
		Hits:            hits,
		Notes:           notes(len(hits) == 0),
	}
}

func notes(clean bool) []string {
	n := []string{
		"Layer A only: invisible/format Unicode and space homoglyphs (edit-based carriers).",
		"Statistical (token-sampling) watermarks are not detectable here; they need a rewrite.",
		"Inspect kinds: strip, bidi, tag_chars, variation_selector, zwj_family, private_use, space, confusable, other_cf.",
		"Load-bearing invisibles are preserved by default when cleaning: emoji glue, CJK/Mongolian variation selectors, script joiners, complete flag tag sequences, same-script fillers/selectors, RTL directional marks and paired embeddings, and orthographic Arabic/Syriac Cf marks. Use the explicit strip flags only after review.",
	}
	if clean {
		n = append(n, "No deterministic Layer A carriers detected; statistical and pixel-domain marks are out of scope here.")
	}
	return n
}
