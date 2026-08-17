package marks

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func visible(t *testing.T, in string, mode VisibleMode) string {
	t.Helper()
	out, _ := Clean(in, Options{Visible: mode})
	return out
}

func TestVisibleASCIITransliterates(t *testing.T) {
	// The worked example from the design decision.
	got := visible(t, "“Café” — naïve… \U0001F600 中", VisibleASCII)
	// Both dropped characters leave their surrounding spaces behind. Nothing
	// else in this tool collapses whitespace runs, and a "keep the visible
	// characters" pass is the wrong place to start silently reflowing text.
	want := `"Cafe" - naive...  `
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestVisibleASCIIFolds(t *testing.T) {
	cases := map[string]string{
		"éèêë":        "eeee",  // accents stripped
		"Æsop":        "AEsop", // no decomposition, mapped
		"straße":      "strasse",
		"Łódź":        "Lodz",
		"ﬁle":         "file",   // ligature expanded by NFKD
		"café’s":      "cafe's", // curly apostrophe
		"a–b—c":       "a-b-c",
		"½":           "1/2",
		"™":           "TM",
		"© 2026":      "(c) 2026",
		"x → y":       "x -> y",
		"plain ascii": "plain ascii",
	}
	for in, want := range cases {
		if got := visible(t, in, VisibleASCII); got != want {
			t.Errorf("ascii(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVisibleASCIIOutputIsASCII(t *testing.T) {
	in := "中文 مرحبا \U0001F600 café “q”\n"
	got := visible(t, in, VisibleASCII)
	for i, r := range got {
		if r > 0x7E || (r < 0x20 && r != '\n' && r != '\r' && r != '\t') {
			t.Errorf("non-ASCII rune %U at %d in %q", r, i, got)
		}
	}
}

func TestVisibleUTF8KeepsScriptsDropsEmoji(t *testing.T) {
	in := "中文 café \U0001F600\U0001F468‍\U0001F469 done\n"
	got := visible(t, in, VisibleUTF8)
	for _, want := range []string{"中文", "café", "done"} {
		if !strings.Contains(got, want) {
			t.Errorf("lost %q from %q", want, got)
		}
	}
	for _, unwanted := range []rune{0x1F600, 0x1F468, 0x200D} {
		if strings.ContainsRune(got, unwanted) {
			t.Errorf("kept %U in %q", unwanted, got)
		}
	}
}

func TestVisibleKeepsAllowedWhitespaceOnly(t *testing.T) {
	// Tab, CR and LF survive; every other space-like rune folds to one space.
	in := "a\tb\r\nc d e f"
	want := "a\tb\r\nc d e\nf"
	if got := visible(t, in, VisibleUTF8); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestVisibleDropsControlCharacters(t *testing.T) {
	in := "a\x00b\x07c\x1bd"
	if got := visible(t, in, VisibleUTF8); got != "abcd" {
		t.Errorf("got %q, want %q", got, "abcd")
	}
}

func TestVisibleDropsInvalidUTF8(t *testing.T) {
	in := "ok\xff\xfe bytes"
	got := visible(t, in, VisibleUTF8)
	if got != "ok bytes" {
		t.Errorf("got %q, want %q", got, "ok bytes")
	}
	if !utf8.ValidString(got) {
		t.Error("output is not valid UTF-8")
	}
}

func TestVisibleKeepsGenuineReplacementChar(t *testing.T) {
	// A real U+FFFD in the input is a printable character, unlike a stray
	// byte that merely decodes to one.
	if got := visible(t, "a�b", VisibleUTF8); got != "a�b" {
		t.Errorf("got %q, want the U+FFFD preserved", got)
	}
}

func TestVisibleKeepsBMPSymbols(t *testing.T) {
	// Box drawing, check marks and old-school symbols are visible text and
	// must survive a "keep the visible characters" pass.
	in := "┌─┐ ✓ ★ ☎ °C"
	if got := visible(t, in, VisibleUTF8); got != in {
		t.Errorf("got %q, want unchanged", got)
	}
}

func TestVisibleOverridesPreservationRules(t *testing.T) {
	// Default cleaning preserves these; forcing visible output must not.
	for _, in := range []string{
		"\U0001F468‍\U0001F469", // emoji ZWJ sequence
		"a‏b",                   // RTL mark
		"می‌ر",                  // Persian ZWNJ
	} {
		got := visible(t, in, VisibleUTF8)
		for _, r := range got {
			if r == 0x200C || r == 0x200D || r == 0x200F {
				t.Errorf("visible pass kept invisible %U in %q", r, got)
			}
		}
	}
}

func TestVisibleStats(t *testing.T) {
	_, st := Clean("café \U0001F600", Options{Visible: VisibleASCII})
	if st.VisibleTransliterated != 1 {
		t.Errorf("VisibleTransliterated = %d, want 1 (the e-acute)", st.VisibleTransliterated)
	}
	if st.VisibleDropped != 1 {
		t.Errorf("VisibleDropped = %d, want 1 (the emoji)", st.VisibleDropped)
	}
	// The NBSP is normalized by the Layer A pass before the visible pass ever
	// sees it, so it lands in ReplacedCount rather than in the folded-space
	// tally. Both passes report separately on purpose.
	if st.ReplacedCount != 1 {
		t.Errorf("ReplacedCount = %d, want 1 (the NBSP)", st.ReplacedCount)
	}
	if st.VisibleSpacesFolded != 0 {
		t.Errorf("VisibleSpacesFolded = %d, want 0", st.VisibleSpacesFolded)
	}
	if !st.Changed() {
		t.Error("Changed() = false after a visible pass made changes")
	}
}

func TestVisibleFoldsSpacesWhenLayerAKeepsThem(t *testing.T) {
	// With KeepSpaces the Layer A pass leaves exotic spaces alone, so the
	// visible pass is what folds them down to U+0020.
	out, st := Clean("a b", Options{Visible: VisibleUTF8, KeepSpaces: true})
	if out != "a b" {
		t.Errorf("got %q, want %q", out, "a b")
	}
	if st.VisibleSpacesFolded != 1 {
		t.Errorf("VisibleSpacesFolded = %d, want 1", st.VisibleSpacesFolded)
	}
}

func TestVisibleOffIsDefault(t *testing.T) {
	in := "café \U0001F600 中\n"
	if got := visible(t, in, VisibleOff); got != in {
		t.Errorf("got %q, want unchanged when the mode is off", got)
	}
}

func TestVisibleIsIdempotent(t *testing.T) {
	in := "“Café” — naïve… \U0001F600 中\ttab\r\n"
	for _, mode := range []VisibleMode{VisibleUTF8, VisibleASCII} {
		once := visible(t, in, mode)
		if twice := visible(t, once, mode); twice != once {
			t.Errorf("%s: second pass changed the text: %q -> %q", mode, once, twice)
		}
	}
}

func TestVisibleModeValid(t *testing.T) {
	for _, m := range []VisibleMode{VisibleOff, VisibleUTF8, VisibleASCII} {
		if !m.Valid() {
			t.Errorf("%q reported invalid", m)
		}
	}
	if VisibleMode("utf-8").Valid() {
		t.Error(`"utf-8" should not be accepted; the mode name is "utf8"`)
	}
}
