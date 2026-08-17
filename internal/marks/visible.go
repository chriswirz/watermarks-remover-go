package marks

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// VisibleMode reduces text to characters that actually show up on screen.
//
// The Layer A scrub is surgical: it removes known carriers and preserves
// anything load-bearing. This is the blunt instrument for when you do not
// want to reason about which invisible is legitimate. Nothing survives but
// visible glyphs and the four whitespace characters below. It therefore
// overrides the preservation rules on purpose: emoji joiners, directional
// marks and script joiners are all invisible, so they all go.
type VisibleMode string

const (
	// VisibleOff leaves the visible-character pass disabled.
	VisibleOff VisibleMode = ""
	// VisibleUTF8 keeps printable Unicode from any script, dropping emoji,
	// invisibles, control characters and invalid UTF-8.
	VisibleUTF8 VisibleMode = "utf8"
	// VisibleASCII additionally folds the text down to printable ASCII,
	// transliterating what it can and dropping the rest.
	VisibleASCII VisibleMode = "ascii"
)

// Valid reports whether m is a mode this package understands.
func (m VisibleMode) Valid() bool {
	switch m {
	case VisibleOff, VisibleUTF8, VisibleASCII:
		return true
	}
	return false
}

// isAllowedWhitespace reports the whitespace that survives a visible pass:
// a plain space, tab, and the two line-ending characters. Tab is kept because
// it is structural in code, Makefiles and TSV. Every other space-like rune is
// folded to a single U+0020 rather than dropped, so words stay separated.
func isAllowedWhitespace(r rune) bool {
	switch r {
	case ' ', '\t', '\r', '\n':
		return true
	}
	return false
}

// isEmojiPlane reports whether r is in the supplementary pictographic blocks.
//
// The test is deliberately limited to the astral planes. BMP symbols such as
// ★, ✓, ☎ and the box-drawing characters are genuinely visible text that
// people use in prose, tables and ASCII art; dropping them under a "keep the
// visible characters" flag would be a nasty surprise.
func isEmojiPlane(r rune) bool {
	return r >= 0x1F000 && r <= 0x1FAFF
}

// isEmojiMechanism reports the invisible or combining runes that exist only to
// assemble emoji sequences: the joiner, the presentation selectors, the
// regional-indicator pairs behind flags, and the keycap enclosure.
func isEmojiMechanism(r rune) bool {
	switch {
	case r == 0x200D: // zero width joiner
		return true
	case r == 0xFE0E || r == 0xFE0F: // text / emoji presentation selectors
		return true
	case r == 0x20E3: // combining enclosing keycap
		return true
	case r >= 0x1F1E6 && r <= 0x1F1FF: // regional indicators
		return true
	}
	return false
}

// dropInvalidUTF8 removes bytes that are not valid UTF-8, returning the count.
//
// This has to run on the raw input, before anything converts the string to
// runes: that conversion silently rewrites every stray byte to U+FFFD, after
// which an encoding error is indistinguishable from a replacement character
// the author actually typed. Forcing visible text means forcing well-formed
// text, so the modes call this first.
func dropInvalidUTF8(text string) (string, int) {
	if utf8.ValidString(text) {
		return text, 0
	}
	var b strings.Builder
	b.Grow(len(text))
	dropped := 0
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == utf8.RuneError && size == 1 {
			dropped++
			i++
			continue
		}
		b.WriteString(text[i : i+size])
		i += size
	}
	return b.String(), dropped
}

// visibleStats records what the visible pass changed.
type visibleStats struct {
	dropped        int
	transliterated int
	spacesFolded   int
}

// applyVisible rewrites text so that every rune is either visibly rendered or
// one of the four allowed whitespace characters.
func applyVisible(text string, mode VisibleMode) (string, visibleStats) {
	var st visibleStats
	var b strings.Builder
	b.Grow(len(text))

	for _, r := range text {
		if isAllowedWhitespace(r) {
			b.WriteRune(r)
			continue
		}

		// Line and paragraph separators are line breaks written the hard
		// way; keep the break rather than losing the structure.
		if unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) {
			b.WriteByte('\n')
			st.spacesFolded++
			continue
		}
		// Every other space-like rune folds to a single plain space.
		if unicode.Is(unicode.Zs, r) {
			b.WriteByte(' ')
			st.spacesFolded++
			continue
		}

		if isEmojiPlane(r) || isEmojiMechanism(r) {
			st.dropped++
			continue
		}

		if mode == VisibleASCII {
			ascii, ok := toASCII(r)
			switch {
			case !ok:
				st.dropped++
			case ascii == string(r):
				b.WriteString(ascii) // already ASCII, not a substitution
			default:
				b.WriteString(ascii)
				st.transliterated++
			}
			continue
		}

		// VisibleUTF8: keep anything that renders. unicode.IsPrint covers
		// letters, marks, numbers, punctuation and symbols, and excludes the
		// control, format, surrogate, private-use and unassigned categories
		// that this mode exists to remove.
		if unicode.IsPrint(r) {
			b.WriteRune(r)
			continue
		}
		st.dropped++
	}

	return b.String(), st
}

// toASCII maps a rune to an ASCII equivalent. The bool is false when there is
// no sensible one and the caller should drop the rune.
func toASCII(r rune) (string, bool) {
	if r >= 0x20 && r <= 0x7E {
		return string(r), true
	}
	if s, ok := asciiFolds[r]; ok {
		return s, true
	}

	// Compatibility decomposition turns an accented letter into its base plus
	// combining marks (é -> e + U+0301) and expands ligatures and many symbols
	// (ﬁ -> fi, ™ -> TM, ½ -> 1⁄2). Dropping the marks and keeping the result
	// only when it is fully ASCII gives a readable fold without inventing
	// spellings for scripts that have no Latin form.
	decomposed := norm.NFKD.String(string(r))
	var b strings.Builder
	for _, d := range decomposed {
		if unicode.Is(unicode.Mn, d) {
			continue // combining mark left over from the decomposition
		}
		if s, ok := asciiFolds[d]; ok {
			b.WriteString(s)
			continue
		}
		if d < 0x20 || d > 0x7E {
			return "", false // decomposed to something still not ASCII
		}
		b.WriteRune(d)
	}
	if b.Len() == 0 {
		return "", false
	}
	return b.String(), true
}

// asciiFolds covers the characters NFKD leaves alone but that have an obvious
// ASCII spelling: quotation marks, dashes, and the Latin letters whose ASCII
// form is a different letter rather than a stripped accent.
var asciiFolds = map[rune]string{
	// Quotation marks and apostrophes.
	0x2018: "'", 0x2019: "'", 0x201A: "'", 0x201B: "'",
	0x201C: `"`, 0x201D: `"`, 0x201E: `"`, 0x201F: `"`,
	0x2032: "'", 0x2033: `"`, 0x00AB: `"`, 0x00BB: `"`,
	0x2039: "'", 0x203A: "'",

	// Dashes, hyphens and the fraction slash NFKD introduces.
	0x2010: "-", 0x2011: "-", 0x2012: "-", 0x2013: "-", 0x2014: "-",
	0x2015: "-", 0x2212: "-", 0x2044: "/",

	// Latin letters with no decomposition.
	0x00C6: "AE", 0x00E6: "ae",
	0x00D8: "O", 0x00F8: "o",
	0x00DF: "ss",
	0x00D0: "D", 0x00F0: "d",
	0x00DE: "Th", 0x00FE: "th",
	0x0110: "D", 0x0111: "d",
	0x0126: "H", 0x0127: "h",
	0x0141: "L", 0x0142: "l",
	0x014A: "N", 0x014B: "n",
	0x0152: "OE", 0x0153: "oe",
	0x0166: "T", 0x0167: "t",

	// Common symbols with a conventional ASCII spelling.
	0x00A9: "(c)", 0x00AE: "(r)",
	0x2022: "*", 0x00B7: "*",
	0x2192: "->", 0x2190: "<-", 0x21D2: "=>",
	0x2260: "!=", 0x2264: "<=", 0x2265: ">=",
}
