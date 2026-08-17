package marks

import "fmt"

// scan walks rs once, resolving each rune against its neighbourhood, and
// hands every decision to visit. Inspect and Clean differ only in what they
// do with the decisions, so the traversal (including the prevKept cursor
// rules that keep emoji and flag sequences bound) lives here alone.
func scan(rs []rune, opt Options, visit func(i int, r rune, d decision)) {
	flagTags := flagTagIndices(rs)
	bidiPairs := bidiEmbeddingIndices(rs)

	var prevKept rune
	for i, r := range rs {
		ctx := context{
			prevKept:           prevKept,
			validFlagTag:       flagTags[i],
			validBidiEmbedding: bidiPairs[i],
		}
		if i > 0 {
			ctx.prevIn = rs[i-1]
		}
		if i+1 < len(rs) {
			ctx.nextIn = rs[i+1]
		}

		d := decide(r, ctx, opt)
		visit(i, r, d)

		switch d.act {
		case actionKeep:
			// Glue does not advance the cursor: a ZWJ chain (❤️‍🔥) and a flag
			// run must both still see the base that opened them.
			if !isGlue(r) {
				prevKept = d.out
			}
		case actionReplace:
			prevKept = d.out
		case actionStrip:
			// prevKept unchanged: a stripped carrier must not hide the base.
		}
	}
}

func sprintCodepoint(r rune) string { return fmt.Sprintf("U+%04X", r) }
