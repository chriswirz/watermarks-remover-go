package marks

import "unicode"

// Kind classifies a suspicious rune for the inspect report.
type Kind string

const (
	KindStrip             Kind = "strip"
	KindBidi              Kind = "bidi"
	KindTagChars          Kind = "tag_chars"
	KindVariationSelector Kind = "variation_selector"
	KindZWJFamily         Kind = "zwj_family"
	KindPrivateUse        Kind = "private_use"
	KindSpace             Kind = "space"
	KindConfusable        Kind = "confusable"
	KindOtherCf           Kind = "other_cf"
)

// Confidence reports how strong a hit is. Layer A hits are edit-based
// carriers; space homoglyphs occur naturally often enough to be context only.
func (k Kind) Confidence() string {
	if k == KindSpace {
		return "informational"
	}
	return "probable"
}

const (
	vsSupplementLo = 0xE0100 // VS17..VS256
	vsSupplementHi = 0xE01EF
	tagLo          = 0xE0001 // language tag + tag chars
	tagHi          = 0xE007F
)

func isPrivateUse(r rune) bool {
	return (r >= 0xE000 && r <= 0xF8FF) ||
		(r >= 0xF0000 && r <= 0xFFFFD) ||
		(r >= 0x100000 && r <= 0x10FFFD)
}

func isVariationSelector(r rune) bool {
	return (r >= vsSupplementLo && r <= vsSupplementHi) ||
		(r >= 0xFE00 && r <= 0xFE0F) ||
		(r >= 0x180B && r <= 0x180D)
}

func isStripRune(r rune) bool {
	if _, ok := stripRunes[r]; ok {
		return true
	}
	return (r >= vsSupplementLo && r <= vsSupplementHi) ||
		(r >= tagLo && r <= tagHi) ||
		isPrivateUse(r)
}

func stripKind(r rune) Kind {
	switch {
	case r >= tagLo && r <= tagHi:
		return KindTagChars
	case isVariationSelector(r):
		return KindVariationSelector
	case bidiRunes[r]:
		return KindBidi
	case zeroWidthFamily[r]:
		return KindZWJFamily
	case isPrivateUse(r):
		return KindPrivateUse
	}
	return KindStrip
}

// isEmojiBase reports whether r can start or continue an emoji sequence.
// Deliberately broad: a false positive only preserves an invisible that a
// paranoid run would have stripped, while a false negative mangles an emoji.
func isEmojiBase(r rune) bool {
	switch {
	case r >= 0x1F000 && r <= 0x1FAFF:
		return true
	case r >= 0x2190 && r <= 0x25FF: // arrows, technical, enclosed
		return true
	case r >= 0x2600 && r <= 0x27BF: // misc symbols, dingbats
		return true
	case r >= 0x2B00 && r <= 0x2BFF:
		return true
	case r == 0x00A9 || r == 0x00AE || r == 0x2122:
		return true
	case r == 0x3030 || r == 0x303D || r == 0x3297 || r == 0x3299:
		return true
	case r == '#' || r == '*' || (r >= '0' && r <= '9'): // keycap bases
		return true
	}
	return false
}

func isCJKIdeograph(r rune) bool {
	return (r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0x20000 && r <= 0x323AF)
}

func isMongolianBase(r rune) bool { return r >= 0x1800 && r <= 0x18AF }

func isMongolianLetter(r rune) bool {
	return isMongolianBase(r) && unicode.IsLetter(r)
}

func isKhmerLetter(r rune) bool {
	return r >= 0x1780 && r <= 0x17FF && unicode.IsLetter(r)
}

func isHangulJamo(r rune) bool {
	return (r >= 0x1100 && r <= 0x11FF) ||
		(r >= 0xA960 && r <= 0xA97C) || // Jamo Extended-A
		(r >= 0xD7B0 && r <= 0xD7C6) // Jamo Extended-B
}

func isTagChar(r rune) bool { return r >= 0xE0020 && r <= 0xE007F }

// joiningScript names a broad script group in which ZWJ/ZWNJ can be
// orthographic. Empty means the rune is not such a base.
func joiningScript(r rune) string {
	ranges := []struct {
		lo, hi rune
		name   string
	}{
		{0x0600, 0x08FF, "arabic"},
		{0x0900, 0x0DFF, "indic"},
		{0x0F00, 0x109F, "south-asian"},
		{0x1780, 0x17FF, "khmer"},
		{0x1800, 0x18AF, "mongolian"},
	}
	for _, rg := range ranges {
		if r >= rg.lo && r <= rg.hi && (unicode.IsLetter(r) || unicode.IsMark(r)) {
			return rg.name
		}
	}
	return ""
}

// isGlue reports whether r is a load-bearing invisible: emoji glue, a script
// joiner, a flag tag char, or a same-script filler/selector. Glue never
// advances the "previous kept base" cursor, so ZWJ chains and flag runs stay
// bound to the base that opened them.
func isGlue(r rune) bool {
	return emojiGlue[r] || isVariationSelector(r) || scriptJoiners[r] ||
		isTagChar(r) || mongolianFVS[r] || khmerVowels[r] || hangulFillers[r]
}
