package marks

import (
	"strings"
	"testing"
)

func cleanDefault(t *testing.T, in string) string {
	t.Helper()
	out, _ := Clean(in, Options{})
	return out
}

func TestStripsInvisibleCarriers(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"zero width space", "he\u200bllo", "hello"},
		{"soft hyphen", "he\u00adllo", "hello"},
		{"word joiner", "a\u2060b", "ab"},
		{"bom", "\ufeffhello", "hello"},
		{"tag chars", "hi\U000E0041\U000E0049", "hi"},
		{"private use", "hi\ue000there", "hithere"},
		{"rlo override", "file\u202egnp.exe", "filegnp.exe"},
		{"nbsp normalized", "a\u00a0b", "a b"},
		{"em space normalized", "a\u2003b", "a b"},
		{"plain text untouched", "Hello, world!\n", "Hello, world!\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cleanDefault(t, tc.in); got != tc.want {
				t.Errorf("Clean(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestPreservesLoadBearingInvisibles(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		// A ZWJ between two emoji builds one glyph; stripping it splits it.
		{"emoji zwj family", "\U0001F468\u200d\U0001F469\u200d\U0001F467"},
		{"emoji variation selector", "\u2696\ufe0f"},
		{"heart on fire", "\u2764\ufe0f\u200d\U0001F525"},
		// ZWNJ inside Persian is orthographic, not a carrier.
		{"persian zwnj", "\u0645\u06cc\u200c\u0631\u0648\u0645"},
		// A complete subdivision flag is base + tag chars + cancel tag.
		{"scotland flag", "\U0001F3F4\U000E0067\U000E0062\U000E0073\U000E0063\U000E0074\U000E007F"},
		// Directional marks are legitimate in mixed RTL/LTR prose.
		{"rtl mark", "price: \u200f\u0645\u0631\u062d\u0628\u0627\u200e ok"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cleanDefault(t, tc.in); got != tc.in {
				t.Errorf("Clean(%q) altered load-bearing text: got %q", tc.in, got)
			}
		})
	}
}

func TestStripEmojiGlueIsParanoid(t *testing.T) {
	in := "\U0001F468\u200d\U0001F469"
	got, st := Clean(in, Options{StripEmojiGlue: true})
	if strings.ContainsRune(got, 0x200D) {
		t.Errorf("StripEmojiGlue kept the ZWJ: %q", got)
	}
	if st.RemovedCount != 1 {
		t.Errorf("RemovedCount = %d, want 1", st.RemovedCount)
	}
}

func TestStripBidiRemovesPreservedMarks(t *testing.T) {
	in := "a\u200fb"
	if got := cleanDefault(t, in); got != in {
		t.Fatalf("default clean should keep RLM, got %q", got)
	}
	if got, _ := Clean(in, Options{StripBidi: true}); got != "ab" {
		t.Errorf("StripBidi: got %q, want %q", got, "ab")
	}
}

func TestUnpairedEmbeddingIsStripped(t *testing.T) {
	// A balanced LRE\u2026PDF pair is legitimate formatting; a dangling opener is not.
	paired := "a\u202ab\u202cc"
	if got := cleanDefault(t, paired); got != paired {
		t.Errorf("balanced embedding stripped: %q", got)
	}
	if got := cleanDefault(t, "a\u202ab"); got != "ab" {
		t.Errorf("dangling LRE kept: %q", got)
	}
}

func TestIncompleteFlagTagsAreStripped(t *testing.T) {
	// Tag chars without the terminating U+E007F are contraband, not a flag.
	if got := cleanDefault(t, "\U0001F3F4\U000E0067\U000E0062"); got != "\U0001F3F4" {
		t.Errorf("incomplete flag tags kept: %q", got)
	}
}

func TestAggressiveHomoglyphs(t *testing.T) {
	in := "R\u0435st" // Cyrillic small ye
	if got := cleanDefault(t, in); got != in {
		t.Errorf("confusables mapped without the flag: %q", got)
	}
	got, st := Clean(in, Options{AggressiveHomoglyphs: true})
	if got != "Rest" {
		t.Errorf("got %q, want %q", got, "Rest")
	}
	if st.ReplacedCount != 1 {
		t.Errorf("ReplacedCount = %d, want 1", st.ReplacedCount)
	}
}

func TestKeepSpaces(t *testing.T) {
	in := "a\u00a0b"
	got, st := Clean(in, Options{KeepSpaces: true})
	if got != in {
		t.Errorf("KeepSpaces rewrote a space: %q", got)
	}
	if st.Changed() {
		t.Errorf("KeepSpaces reported a change: %+v", st)
	}
}

func TestNFKC(t *testing.T) {
	got, st := Clean("\ufb01le \u2460", Options{NFKC: true})
	if got != "file 1" {
		t.Errorf("got %q, want %q", got, "file 1")
	}
	if !st.NFKCChanged {
		t.Error("NFKCChanged = false")
	}
}

func TestStatsCountRunesNotBytes(t *testing.T) {
	_, st := Clean("h\u00e9llo\u200b", Options{})
	if st.InputLength != 6 {
		t.Errorf("InputLength = %d, want 6 runes", st.InputLength)
	}
	if st.OutputLength != 5 {
		t.Errorf("OutputLength = %d, want 5 runes", st.OutputLength)
	}
}

func TestInspectMatchesClean(t *testing.T) {
	// Whatever inspect flags at default settings, clean must actually remove
	// (bidi excepted: inspect reports it, clean preserves it by design).
	in := "a\u200bb\u00a0c\ue000d"
	rep := Inspect(in, InspectOptions{})
	if rep.SuspiciousTotal != 3 {
		t.Fatalf("SuspiciousTotal = %d, want 3: %+v", rep.SuspiciousTotal, rep.Hits)
	}
	out, st := Clean(in, Options{})
	if st.RemovedCount+st.ReplacedCount != 3 {
		t.Errorf("clean touched %d runes, inspect found 3", st.RemovedCount+st.ReplacedCount)
	}
	if out != "ab cd" {
		t.Errorf("got %q, want %q", out, "ab cd")
	}
}

func TestInspectKinds(t *testing.T) {
	rep := Inspect("\u200b\u202e\U000E0041\ue000\u00a0", InspectOptions{})
	want := map[Kind]bool{
		KindZWJFamily: true, KindBidi: true, KindTagChars: true,
		KindPrivateUse: true, KindSpace: true,
	}
	for _, h := range rep.Hits {
		if !want[h.Kind] {
			t.Errorf("unexpected kind %q for %s", h.Kind, h.Label)
		}
		delete(want, h.Kind)
	}
	if len(want) != 0 {
		t.Errorf("kinds not reported: %v", want)
	}
}

func TestInspectOffsetsAreRuneIndices(t *testing.T) {
	rep := Inspect("h\u00e9llo\u200b", InspectOptions{})
	if len(rep.Hits) != 1 || rep.Hits[0].SampleOffsets[0] != 5 {
		t.Errorf("want a single hit at rune offset 5, got %+v", rep.Hits)
	}
}

func TestInspectOrdersByFrequency(t *testing.T) {
	rep := Inspect("\u00a0a\u200b\u200b\u200b", InspectOptions{})
	if len(rep.Hits) != 2 || rep.Hits[0].Count != 3 {
		t.Fatalf("want the 3x hit first, got %+v", rep.Hits)
	}
}

func TestCleanTextIsIdempotent(t *testing.T) {
	in := "a\u200b b\u00a0c \U0001F468\u200d\U0001F469"
	once := cleanDefault(t, in)
	if twice := cleanDefault(t, once); twice != once {
		t.Errorf("second clean changed the text: %q -> %q", once, twice)
	}
}

func TestEmptyInput(t *testing.T) {
	out, st := Clean("", Options{})
	if out != "" || st.Changed() {
		t.Errorf("empty input: got %q %+v", out, st)
	}
	if rep := Inspect("", InspectOptions{}); rep.SuspiciousTotal != 0 || len(rep.Hits) != 0 {
		t.Errorf("empty inspect: %+v", rep)
	}
}
