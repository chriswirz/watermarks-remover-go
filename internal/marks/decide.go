package marks

import (
	"fmt"
	"unicode"
)

// Options control which classes of invisible are treated as contraband.
// The zero value is the conservative default used by Clean: normalize spaces,
// preserve every load-bearing invisible, leave directional marks alone.
type Options struct {
	// NFKC applies Unicode NFKC normalization after the scrub.
	NFKC bool
	// AggressiveHomoglyphs maps Cyrillic/fullwidth Latin lookalikes to ASCII.
	AggressiveHomoglyphs bool
	// KeepSpaces leaves exotic spaces as-is instead of rewriting them to U+0020.
	KeepSpaces bool
	// StripEmojiGlue is the paranoid mode: strip load-bearing invisibles too.
	StripEmojiGlue bool
	// StripBidi also strips legitimate RTL/LTR directional marks and isolates.
	StripBidi bool
	// Visible, when set, additionally reduces the text to visible characters
	// plus space, tab, CR and LF. See VisibleMode.
	Visible VisibleMode
}

type action int

const (
	actionKeep action = iota
	actionStrip
	actionReplace
)

// decision is the shared verdict for one rune. Inspect and Clean run the same
// classifier so a hit reported by one is exactly what the other acts on.
type decision struct {
	act  action
	out  rune // surviving rune for keep/replace
	kind Kind // classification; empty when not suspicious
}

// context is the neighbourhood a decision depends on. prevKept is the last
// surviving non-glue rune; prevIn and nextIn are raw input neighbours (0 at
// the edges).
type context struct {
	prevKept, prevIn, nextIn rune
	validFlagTag             bool
	validBidiEmbedding       bool
}

func decide(r rune, ctx context, opt Options) decision {
	keep := decision{act: actionKeep, out: r}

	if !opt.StripBidi {
		if ctx.validBidiEmbedding || preservableBidiRunes[r] {
			return keep
		}
	}

	if !opt.StripEmojiGlue {
		if d, ok := decideGlue(r, ctx); ok {
			return d
		}
	}

	if isStripRune(r) {
		return decision{act: actionStrip, kind: stripKind(r)}
	}
	if !opt.KeepSpaces {
		if _, ok := spaceHomoglyphs[r]; ok {
			return decision{act: actionReplace, out: ' ', kind: KindSpace}
		}
	}
	if opt.AggressiveHomoglyphs {
		if ascii, ok := latinConfusables[r]; ok {
			return decision{act: actionReplace, out: ascii, kind: KindConfusable}
		}
	}
	if _, isSpace := spaceHomoglyphs[r]; unicode.Is(unicode.Cf, r) && !isSpace {
		return decision{act: actionStrip, kind: KindOtherCf}
	}
	return keep
}

// decideGlue handles the load-bearing cases: an invisible that is part of a
// visible sequence in its own context. Reports false when r is not glue here,
// leaving the caller to fall through to the contraband checks.
func decideGlue(r rune, ctx context) (decision, bool) {
	keep := decision{act: actionKeep, out: r}

	// A variation selector or Mongolian FVS directly after a base of its own
	// script selects that base's glyph.
	if ctx.prevIn != 0 {
		switch {
		case r >= vsSupplementLo && r <= vsSupplementHi && isCJKIdeograph(ctx.prevIn):
			return keep, true
		case r >= 0x180B && r <= 0x180D && isMongolianBase(ctx.prevIn):
			return keep, true
		case r >= 0xFE00 && r <= 0xFE0D && isCJKIdeograph(ctx.prevIn):
			return keep, true
		}
	}

	if emojiGlue[r] {
		// VS15/VS16 select text vs emoji presentation of the preceding base.
		if (r == 0xFE0E || r == 0xFE0F) && ctx.prevIn != 0 && isEmojiBase(ctx.prevIn) {
			return keep, true
		}
		// ZWJ binds two emoji into one glyph (👨‍👩‍👧).
		if r == 0x200D && isEmojiBase(ctx.prevKept) && isEmojiBase(ctx.nextIn) {
			return keep, true
		}
	}

	// ZWJ/ZWNJ between two letters of the same complex script is orthographic.
	if scriptJoiners[r] && ctx.prevIn != 0 && ctx.nextIn != 0 {
		if s := joiningScript(ctx.prevIn); s != "" && s == joiningScript(ctx.nextIn) {
			return keep, true
		}
	}
	if isTagChar(r) && ctx.validFlagTag {
		return keep, true
	}
	if mongolianFVS[r] && isMongolianLetter(ctx.prevKept) {
		return keep, true
	}
	if khmerVowels[r] && isKhmerLetter(ctx.prevKept) {
		return keep, true
	}
	if hangulFillers[r] && isHangulJamo(ctx.prevKept) {
		return keep, true
	}
	if orthographicCf[r] {
		return keep, true
	}
	return decision{}, false
}

// flagTagIndices returns the rune indices that belong to a complete
// subdivision-flag sequence: U+1F3F4, one or more tag chars, U+E007F (🏴󠁧󠁢󠁳󠁣󠁴󠁿).
// Tag chars outside such a sequence are contraband.
func flagTagIndices(rs []rune) map[int]bool {
	valid := map[int]bool{}
	for i := 0; i < len(rs); {
		if rs[i] != 0x1F3F4 { // waving black flag
			i++
			continue
		}
		j := i + 1
		for j < len(rs) && rs[j] >= 0xE0020 && rs[j] <= 0xE007E {
			j++
		}
		if j > i+1 && j < len(rs) && rs[j] == 0xE007F {
			for k := i + 1; k <= j; k++ {
				valid[k] = true
			}
			i = j + 1
			continue
		}
		i++
	}
	return valid
}

// bidiEmbeddingIndices returns the indices in complete LRE/RLE … PDF pairs.
// Overrides (LRO/RLO) are excluded: they can reorder unrelated spans, so they
// stay strippable even when balanced.
func bidiEmbeddingIndices(rs []rune) map[int]bool {
	valid := map[int]bool{}
	type opener struct {
		r     rune
		index int
	}
	var stack []opener
	for i, r := range rs {
		switch r {
		case 0x202A, 0x202B, 0x202D, 0x202E:
			stack = append(stack, opener{r, i})
		case 0x202C:
			if len(stack) == 0 {
				continue
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if top.r == 0x202A || top.r == 0x202B {
				valid[top.index] = true
				valid[i] = true
			}
		}
	}
	return valid
}

// label renders a rune the way the report shows it: "U+200B ZERO WIDTH SPACE (Cf)".
func label(r rune) string {
	return fmt.Sprintf("U+%04X %s (%s)", r, runeName(r), category(r))
}

func runeName(r rune) string {
	if n, ok := stripRunes[r]; ok {
		return n
	}
	if n, ok := spaceHomoglyphs[r]; ok {
		return n
	}
	switch {
	case r >= vsSupplementLo && r <= vsSupplementHi:
		return fmt.Sprintf("VARIATION SELECTOR-%d", 17+(r-vsSupplementLo))
	case r == 0xE0001:
		return "LANGUAGE TAG"
	case r >= 0xE0020 && r <= 0xE007E:
		return fmt.Sprintf("TAG %q", rune(r-0xE0000))
	case r == 0xE007F:
		return "CANCEL TAG"
	case isPrivateUse(r):
		return "PRIVATE USE CHARACTER"
	case unicode.IsPrint(r):
		return fmt.Sprintf("%q", r)
	}
	return "UNKNOWN"
}

func category(r rune) string {
	for name, table := range map[string]*unicode.RangeTable{
		"Cf": unicode.Cf, "Co": unicode.Co, "Cc": unicode.Cc, "Cs": unicode.Cs,
		"Zs": unicode.Zs, "Zl": unicode.Zl, "Zp": unicode.Zp,
		"Mn": unicode.Mn, "Lu": unicode.Lu, "Ll": unicode.Ll,
	} {
		if unicode.Is(table, r) {
			return name
		}
	}
	return "Cn"
}
