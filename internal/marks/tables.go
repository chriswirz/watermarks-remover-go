package marks

// Codepoint tables for Layer A: invisible/format Unicode and space homoglyphs.
//
// Every rune listed here is either invisible when rendered or visually
// indistinguishable from an ASCII character, which is what makes them usable
// as edit-based carriers. Names are kept alongside the values because Go has
// no Unicode name database in the standard library and the human report wants
// to say "U+200B ZERO WIDTH SPACE" rather than a bare number.

// stripRunes are format/invisible controls commonly used for steganography or
// left behind by broken pastes.
var stripRunes = map[rune]string{
	0x00AD: "SOFT HYPHEN",
	0x034F: "COMBINING GRAPHEME JOINER",
	0x061C: "ARABIC LETTER MARK",
	0x115F: "HANGUL CHOSEONG FILLER",
	0x1160: "HANGUL JUNGSEONG FILLER",
	0x17B4: "KHMER VOWEL INHERENT AQ",
	0x17B5: "KHMER VOWEL INHERENT AA",
	0x180B: "MONGOLIAN FREE VARIATION SELECTOR ONE",
	0x180C: "MONGOLIAN FREE VARIATION SELECTOR TWO",
	0x180D: "MONGOLIAN FREE VARIATION SELECTOR THREE",
	0x180E: "MONGOLIAN VOWEL SEPARATOR",
	0x200B: "ZERO WIDTH SPACE",
	0x200C: "ZERO WIDTH NON-JOINER",
	0x200D: "ZERO WIDTH JOINER",
	0x200E: "LEFT-TO-RIGHT MARK",
	0x200F: "RIGHT-TO-LEFT MARK",
	0x202A: "LEFT-TO-RIGHT EMBEDDING",
	0x202B: "RIGHT-TO-LEFT EMBEDDING",
	0x202C: "POP DIRECTIONAL FORMATTING",
	0x202D: "LEFT-TO-RIGHT OVERRIDE",
	0x202E: "RIGHT-TO-LEFT OVERRIDE",
	0x2060: "WORD JOINER",
	0x2061: "FUNCTION APPLICATION",
	0x2062: "INVISIBLE TIMES",
	0x2063: "INVISIBLE SEPARATOR",
	0x2064: "INVISIBLE PLUS",
	0x2066: "LEFT-TO-RIGHT ISOLATE",
	0x2067: "RIGHT-TO-LEFT ISOLATE",
	0x2068: "FIRST STRONG ISOLATE",
	0x2069: "POP DIRECTIONAL ISOLATE",
	0x206A: "INHIBIT SYMMETRIC SWAPPING",
	0x206B: "ACTIVATE SYMMETRIC SWAPPING",
	0x206C: "INHIBIT ARABIC FORM SHAPING",
	0x206D: "ACTIVATE ARABIC FORM SHAPING",
	0x206E: "NATIONAL DIGIT SHAPES",
	0x206F: "NOMINAL DIGIT SHAPES",
	0xFE00: "VARIATION SELECTOR-1",
	0xFE01: "VARIATION SELECTOR-2",
	0xFE02: "VARIATION SELECTOR-3",
	0xFE03: "VARIATION SELECTOR-4",
	0xFE04: "VARIATION SELECTOR-5",
	0xFE05: "VARIATION SELECTOR-6",
	0xFE06: "VARIATION SELECTOR-7",
	0xFE07: "VARIATION SELECTOR-8",
	0xFE08: "VARIATION SELECTOR-9",
	0xFE09: "VARIATION SELECTOR-10",
	0xFE0A: "VARIATION SELECTOR-11",
	0xFE0B: "VARIATION SELECTOR-12",
	0xFE0C: "VARIATION SELECTOR-13",
	0xFE0D: "VARIATION SELECTOR-14",
	0xFE0E: "VARIATION SELECTOR-15",
	0xFE0F: "VARIATION SELECTOR-16",
	0xFEFF: "ZERO WIDTH NO-BREAK SPACE (BOM)",
	0xFFF9: "INTERLINEAR ANNOTATION ANCHOR",
	0xFFFA: "INTERLINEAR ANNOTATION SEPARATOR",
	0xFFFB: "INTERLINEAR ANNOTATION TERMINATOR",
}

// spaceHomoglyphs are runes that look like (or substitute for) U+0020.
var spaceHomoglyphs = map[rune]string{
	0x00A0: "NO-BREAK SPACE",
	0x1680: "OGHAM SPACE MARK",
	0x2000: "EN QUAD",
	0x2001: "EM QUAD",
	0x2002: "EN SPACE",
	0x2003: "EM SPACE",
	0x2004: "THREE-PER-EM SPACE",
	0x2005: "FOUR-PER-EM SPACE",
	0x2006: "SIX-PER-EM SPACE",
	0x2007: "FIGURE SPACE",
	0x2008: "PUNCTUATION SPACE",
	0x2009: "THIN SPACE",
	0x200A: "HAIR SPACE",
	0x202F: "NARROW NO-BREAK SPACE",
	0x205F: "MEDIUM MATHEMATICAL SPACE",
	0x3000: "IDEOGRAPHIC SPACE",
}

// latinConfusables are Latin lookalikes, mapped only in aggressive mode: a
// Cyrillic "о" in a Russian word is not a watermark, so this stays opt-in.
var latinConfusables = map[rune]rune{
	0x0410: 'A', 0x0412: 'B', 0x0415: 'E', 0x041A: 'K', 0x041C: 'M',
	0x041D: 'H', 0x041E: 'O', 0x0420: 'P', 0x0421: 'C', 0x0422: 'T',
	0x0425: 'X', 0x0430: 'a', 0x0435: 'e', 0x043E: 'o', 0x0440: 'p',
	0x0441: 'c', 0x0443: 'y', 0x0445: 'x', 0x0456: 'i',
}

func init() {
	// Fullwidth Latin (U+FF21..U+FF5A) maps to ASCII by a constant offset.
	for r := rune(0xFF21); r <= 0xFF3A; r++ {
		latinConfusables[r] = 'A' + (r - 0xFF21)
	}
	for r := rune(0xFF41); r <= 0xFF5A; r++ {
		latinConfusables[r] = 'a' + (r - 0xFF41)
	}
}

// bidiRunes are the directional format controls, a subset of stripRunes that
// inspect labels separately.
var bidiRunes = map[rune]bool{
	0x061C: true, 0x200E: true, 0x200F: true, 0x202A: true, 0x202B: true,
	0x202C: true, 0x202D: true, 0x202E: true, 0x2066: true, 0x2067: true,
	0x2068: true, 0x2069: true,
}

// preservableBidiRunes are legitimate in mixed RTL/LTR prose: inspect reports
// them, but the default clean keeps them. Embeddings and overrides stay
// destructive by default because they can reorder unrelated spans.
var preservableBidiRunes = map[rune]bool{
	0x061C: true, 0x200E: true, 0x200F: true,
	0x2066: true, 0x2067: true, 0x2068: true, 0x2069: true,
}

// zeroWidthFamily is the common edit-based carrier set.
var zeroWidthFamily = map[rune]bool{
	0x200B: true, 0x200C: true, 0x200D: true,
	0x2060: true, 0xFEFF: true, 0x180E: true,
}

// emojiGlue is zero-width joiner plus the text/emoji variation selectors.
// Free-floating they are carriers; after an emoji base they are part of the
// visible sequence (⚖️, 👨‍👩‍👧, ❤️‍🔥) and stripping them alters the text.
var emojiGlue = map[rune]bool{0x200D: true, 0xFE0E: true, 0xFE0F: true}

// scriptJoiners are orthographic inside complex scripts (Persian می‌روم,
// Devanagari क्‍ष) when they sit between two letters of the same script.
var scriptJoiners = map[rune]bool{0x200C: true, 0x200D: true}

var (
	mongolianFVS  = map[rune]bool{0x180B: true, 0x180C: true, 0x180D: true}
	khmerVowels   = map[rune]bool{0x17B4: true, 0x17B5: true}
	hangulFillers = map[rune]bool{0x115F: true, 0x1160: true}
)

// orthographicCf are Cf codepoints that are normal Arabic/Syriac orthography
// rather than carriers.
var orthographicCf = map[rune]bool{
	0x0600: true, 0x0601: true, 0x0602: true, 0x0603: true, 0x0604: true,
	0x0605: true, 0x06DD: true, 0x070F: true, 0x08E2: true,
	0x110BD: true, 0x110CD: true,
}
