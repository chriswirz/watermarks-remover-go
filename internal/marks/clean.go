package marks

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Stats records what a Clean pass changed. Removed and Replaced are keyed by
// the human label of the input rune, so the caller can name what it dropped.
type Stats struct {
	InputLength   int            `json:"input_length"`
	OutputLength  int            `json:"output_length"`
	Removed       map[string]int `json:"removed"`
	Replaced      map[string]int `json:"replaced"`
	RemovedCount  int            `json:"removed_count"`
	ReplacedCount int            `json:"replaced_count"`
	NFKCChanged   bool           `json:"nfkc_changed"`

	// Visible* record the separate visible-character pass, which reports its
	// own totals rather than per-rune labels: on a document full of non-ASCII
	// text the label map would be as large as the document.
	VisibleDropped        int `json:"visible_dropped,omitempty"`
	VisibleTransliterated int `json:"visible_transliterated,omitempty"`
	VisibleSpacesFolded   int `json:"visible_spaces_folded,omitempty"`
}

// Changed reports whether the clean altered the text at all.
func (s Stats) Changed() bool {
	return s.RemovedCount > 0 || s.ReplacedCount > 0 || s.NFKCChanged ||
		s.VisibleDropped > 0 || s.VisibleTransliterated > 0 || s.VisibleSpacesFolded > 0
}

// Clean strips invisible carriers from text and returns the result with a
// record of what it did. Lengths in Stats are rune counts.
func Clean(text string, opt Options) (string, Stats) {
	invalidDropped := 0
	if opt.Visible != VisibleOff {
		// Must happen before the []rune conversion below, which would turn
		// malformed bytes into indistinguishable U+FFFD runes.
		text, invalidDropped = dropInvalidUTF8(text)
	}

	rs := []rune(text)
	stats := Stats{
		InputLength: len(rs),
		Removed:     map[string]int{},
		Replaced:    map[string]int{},
	}

	var b strings.Builder
	b.Grow(len(text))

	scan(rs, opt, func(_ int, r rune, d decision) {
		switch d.act {
		case actionKeep:
			b.WriteRune(d.out)
		case actionReplace:
			b.WriteRune(d.out)
			stats.Replaced[label(r)]++
			stats.ReplacedCount++
		case actionStrip:
			stats.Removed[label(r)]++
			stats.RemovedCount++
		}
	})

	out := b.String()

	// The visible pass runs after the Layer A scrub and before NFKC: the
	// scrub has already removed the carriers it can name, so whatever
	// invisible is left here is exactly what this mode exists to catch.
	if opt.Visible != VisibleOff {
		var vs visibleStats
		out, vs = applyVisible(out, opt.Visible)
		stats.VisibleDropped = vs.dropped + invalidDropped
		stats.VisibleTransliterated = vs.transliterated
		stats.VisibleSpacesFolded = vs.spacesFolded
	}

	if opt.NFKC {
		normalized := norm.NFKC.String(out)
		if normalized != out {
			stats.NFKCChanged = true
			// One aggregate entry: NFKC folds whole sequences (ﬁ → fi,
			// ① → 1), so a per-rune count would misreport the edit.
			stats.Replaced["NFKC_normalize"] += countRuneDiff(out, normalized)
			stats.ReplacedCount += countRuneDiff(out, normalized)
			out = normalized
		}
	}
	stats.OutputLength = len([]rune(out))
	return out, stats
}

// countRuneDiff estimates how many input runes NFKC rewrote, by counting the
// runes outside the common prefix and suffix. Exact edit distance would cost
// more than the number is worth; at minimum one rune changed.
func countRuneDiff(before, after string) int {
	a, b := []rune(before), []rune(after)
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	j := 0
	for j < len(a)-i && j < len(b)-i && a[len(a)-1-j] == b[len(b)-1-j] {
		j++
	}
	if n := len(a) - i - j; n > 0 {
		return n
	}
	return 1
}
