package main

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/chriswirz/watermarks-remover-go/internal/lines"
	"github.com/chriswirz/watermarks-remover-go/internal/marks"
)

func testFlagSet() (*flag.FlagSet, *string, *bool) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(os.NewFile(0, os.DevNull))
	out := fs.String("o", "", "")
	verbose := fs.Bool("v", false, "")
	return fs, out, verbose
}

func TestParseAnywhere(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantOut  string
		wantV    bool
		wantArgs []string
	}{
		{"flags first", []string{"-o", "x.md", "-v", "a.md"}, "x.md", true, []string{"a.md"}},
		{"flags last", []string{"a.md", "-o", "x.md", "-v"}, "x.md", true, []string{"a.md"}},
		{"flags interleaved", []string{"-v", "a.md", "-o", "x.md", "b.md"}, "x.md", true, []string{"a.md", "b.md"}},
		{"equals form", []string{"a.md", "-o=x.md"}, "x.md", false, []string{"a.md"}},
		{"bool does not eat operand", []string{"-v", "a.md"}, "", true, []string{"a.md"}},
		{"no args", nil, "", false, nil},
		{"stdin dash is an operand", []string{"-v", "-"}, "", true, []string{"-"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs, out, verbose := testFlagSet()
			if err := parseAnywhere(fs, tc.args); err != nil {
				t.Fatal(err)
			}
			if *out != tc.wantOut || *verbose != tc.wantV {
				t.Errorf("-o=%q -v=%v, want %q %v", *out, *verbose, tc.wantOut, tc.wantV)
			}
			if got := fs.Args(); !reflect.DeepEqual(got, tc.wantArgs) && len(got)+len(tc.wantArgs) > 0 {
				t.Errorf("operands = %v, want %v", got, tc.wantArgs)
			}
		})
	}
}

func TestParseAnywhereDoubleDash(t *testing.T) {
	// After "--", a leading dash is part of a filename, not a flag.
	fs, out, _ := testFlagSet()
	if err := parseAnywhere(fs, []string{"-o", "x.md", "--", "-weird-name.md"}); err != nil {
		t.Fatal(err)
	}
	if *out != "x.md" {
		t.Errorf("-o = %q", *out)
	}
	if got := fs.Args(); len(got) != 1 || got[0] != "-weird-name.md" {
		t.Errorf("operands = %v", got)
	}
}

// write creates a file in a fresh temp dir and returns its path.
func write(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCleanOneWritesCleanedSibling(t *testing.T) {
	src := write(t, "notes.txt", "he​llo\n")
	if _, err := cleanOne(src, "", marks.Options{}, cleanFlags{}); err != nil {
		t.Fatal(err)
	}
	dest := strings.TrimSuffix(src, ".txt") + ".cleaned.txt"
	if got := read(t, dest); got != "hello\n" {
		t.Errorf("cleaned = %q", got)
	}
	if got := read(t, src); got != "he​llo\n" {
		t.Errorf("source was modified: %q", got)
	}
}

func TestCleanOneInPlaceKeepsBackup(t *testing.T) {
	original := "he​llo\n"
	src := write(t, "notes.md", original)
	if _, err := cleanOne(src, "", marks.Options{}, cleanFlags{inPlace: true}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, src); got != "hello\n" {
		t.Errorf("in-place result = %q", got)
	}
	if got := read(t, src+".bak"); got != original {
		t.Errorf("backup = %q, want the untouched original", got)
	}
}

func TestCleanOneInPlaceRejectsStdin(t *testing.T) {
	_, err := cleanOne("-", "", marks.Options{}, cleanFlags{inPlace: true})
	if err == nil || !strings.Contains(err.Error(), "file path") {
		t.Errorf("err = %v, want a refusal naming the missing path", err)
	}
}

func TestCleanOneAppliesMetadataForMarkdown(t *testing.T) {
	src := write(t, "post.md", "---\ntitle: T\ngenerator: Claude\n---\nBody\n")
	if _, err := cleanOne(src, "", marks.Options{}, cleanFlags{inPlace: true}); err != nil {
		t.Fatal(err)
	}
	got := read(t, src)
	if strings.Contains(got, "Claude") {
		t.Errorf("frontmatter survived:\n%s", got)
	}
	if !strings.Contains(got, "title: T") {
		t.Errorf("unrelated frontmatter dropped:\n%s", got)
	}
}

func TestCleanOneNoMetadataLeavesFrontmatter(t *testing.T) {
	in := "---\ngenerator: Claude\n---\nBody\n"
	src := write(t, "post.md", in)
	if _, err := cleanOne(src, "", marks.Options{}, cleanFlags{inPlace: true, noMeta: true}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, src); got != in {
		t.Errorf("got %q, want unchanged", got)
	}
}

func TestCleanOneRefusesBinary(t *testing.T) {
	src := write(t, "doc.docx", "PK\x03\x04payload")
	_, err := cleanOne(src, "", marks.Options{}, cleanFlags{})
	if err == nil || !strings.Contains(err.Error(), "ZIP container") {
		t.Errorf("err = %v, want a binary refusal", err)
	}
}

func TestRunInspectExitCodes(t *testing.T) {
	clean := write(t, "clean.txt", "plain text\n")
	dirty := write(t, "dirty.txt", "he​llo\n")

	if code, err := runInspect([]string{"-json", clean}); err != nil || code != 0 {
		t.Errorf("clean file: code=%d err=%v, want 0", code, err)
	}
	if code, err := runInspect([]string{"-json", dirty}); err != nil || code != 1 {
		t.Errorf("dirty file: code=%d err=%v, want 1", code, err)
	}
}

func TestRunInspectFlagsMetadataOnlyFiles(t *testing.T) {
	// No invisible runes, but the frontmatter names a generator: still a hit.
	src := write(t, "post.md", "---\ngenerator: Claude\n---\nBody\n")
	code, err := runInspect([]string{"-json", src})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
}

func TestRunCleanRejectsOutputWithMultipleInputs(t *testing.T) {
	a := write(t, "a.txt", "x")
	b := write(t, "b.txt", "y")
	if _, err := runClean([]string{"-o", "out.txt", a, b}); err == nil {
		t.Error("expected a refusal for -o with two inputs")
	}
}

func TestRunCleanRejectsOutputWithInPlace(t *testing.T) {
	a := write(t, "a.txt", "x")
	if _, err := runClean([]string{"-o", "out.txt", "-in-place", a}); err == nil {
		t.Error("expected a refusal for -o together with -in-place")
	}
}

// --- line filtering, recursion, dry run -------------------------------------

func filterFor(t *testing.T, args ...string) lines.Filter {
	t.Helper()
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(os.NewFile(0, os.DevNull))
	opts := registerFilterFlags(fs)
	if err := parseAnywhere(fs, args); err != nil {
		t.Fatal(err)
	}
	f, err := opts.build()
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestFilterIsOffByDefault(t *testing.T) {
	// Line deletion is destructive; it must never happen without being asked.
	if filterFor(t).Active() {
		t.Error("line filter is active with no flags set")
	}
}

func TestAttributionFlagLoadsBuiltins(t *testing.T) {
	f := filterFor(t, "-attribution")
	if !f.Active() || len(f.Patterns) == 0 {
		t.Fatal("-attribution did not load the built-in patterns")
	}
	if _, res := f.Apply("keep\nGenerated by AI\n"); res.Removed != 1 {
		t.Errorf("Removed = %d, want 1", res.Removed)
	}
}

func TestPatternsFlagReplacesBuiltins(t *testing.T) {
	f := filterFor(t, "-attribution", "-patterns", `(?i)^TODO:`)
	out, res := f.Apply("TODO: x\nGenerated by AI\n")
	if res.Removed != 1 {
		t.Errorf("Removed = %d, want 1", res.Removed)
	}
	if !strings.Contains(out, "Generated by AI") {
		t.Error("-patterns should replace the built-in set, not add to it")
	}
}

func TestPatternFileAddsToSet(t *testing.T) {
	dir := t.TempDir()
	pf := filepath.Join(dir, "p.txt")
	if err := os.WriteFile(pf, []byte("(?i)^DRAFT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := filterFor(t, "-attribution", "-pattern-file", pf)
	_, res := f.Apply("DRAFT notice\nGenerated by AI\nkeep\n")
	if res.Removed != 2 {
		t.Errorf("Removed = %d, want 2 (one from each set)", res.Removed)
	}
}

func TestBadPatternIsAnError(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(os.NewFile(0, os.DevNull))
	opts := registerFilterFlags(fs)
	if err := parseAnywhere(fs, []string{"-patterns", "(unclosed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := opts.build(); err == nil {
		t.Error("expected an error for an invalid regexp")
	}
}

func TestCleanRemovesAttributionLines(t *testing.T) {
	src := write(t, "doc.md", "Real content.\nGenerated by AI\nMore content.\n")
	if _, err := cleanOne(src, "", marks.Options{}, cleanFlags{
		inPlace: true,
		filter:  filterFor(t, "-attribution"),
	}); err != nil {
		t.Fatal(err)
	}
	got := read(t, src)
	if strings.Contains(got, "Generated by AI") {
		t.Errorf("attribution line survived:\n%s", got)
	}
	if !strings.Contains(got, "Real content.") || !strings.Contains(got, "More content.") {
		t.Errorf("real content lost:\n%s", got)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	original := "Real content.\nGenerated by AI\nhe​llo\n"
	src := write(t, "doc.md", original)

	changed, err := cleanOne(src, "", marks.Options{}, cleanFlags{
		inPlace: true,
		dryRun:  true,
		filter:  filterFor(t, "-attribution"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("dry run reported no change for a file that has both a mark and an attribution line")
	}
	if got := read(t, src); got != original {
		t.Errorf("dry run modified the file:\n%q", got)
	}
	if _, err := os.Stat(src + ".bak"); err == nil {
		t.Error("dry run created a backup file")
	}
	if _, err := os.Stat(strings.TrimSuffix(src, ".md") + ".cleaned.md"); err == nil {
		t.Error("dry run created a cleaned sibling")
	}
}

func TestCleanRecursesWithFlag(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.md", "sub/b.txt", "sub/deep/c.go"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("he​llo\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A file the walk must not pick up.
	if err := os.WriteFile(filepath.Join(root, "img.png"), []byte("\x89PNG\r\n\x1a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if code, err := runClean([]string{"-in-place", "-r", root}); err != nil || code != 0 {
		t.Fatalf("code=%d err=%v", code, err)
	}
	for _, name := range []string{"a.md", "sub/b.txt", "sub/deep/c.go"} {
		got := read(t, filepath.Join(root, filepath.FromSlash(name)))
		if got != "hello\n" {
			t.Errorf("%s = %q, want %q", name, got, "hello\n")
		}
	}
	if got := read(t, filepath.Join(root, "img.png")); got != "\x89PNG\r\n\x1a\n" {
		t.Error("the walk touched a binary file it should have skipped")
	}
}

func TestExpandTargetsMixesFilesAndDirs(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	loose := write(t, "loose.txt", "y")

	got, err := expandTargets([]string{root, loose, "-"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Errorf("expandTargets = %v, want the walked file plus both explicit args", got)
	}
	if got[len(got)-1] != "-" {
		t.Errorf("stdin argument was not preserved: %v", got)
	}
}

func TestInspectReportsAttributionLines(t *testing.T) {
	src := write(t, "doc.md", "clean line\nGenerated by AI\n")
	// No invisible characters and no metadata: the attribution line alone
	// must be enough to flag the file.
	code, err := runInspect([]string{"-json", "-attribution", src})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if code, _ := runInspect([]string{"-json", src}); code != 0 {
		t.Errorf("code = %d without -attribution, want 0", code)
	}
}

func TestStripCommentsFlag(t *testing.T) {
	src := write(t, "x.go", "package main\n// a comment\ncode := 1\n")
	if _, err := cleanOne(src, "", marks.Options{}, cleanFlags{
		inPlace: true,
		filter:  filterFor(t, "-strip-comments"),
	}); err != nil {
		t.Fatal(err)
	}
	got := read(t, src)
	if strings.Contains(got, "// a comment") {
		t.Errorf("comment survived:\n%s", got)
	}
	if !strings.Contains(got, "package main") {
		t.Errorf("code lost:\n%s", got)
	}
}

// --- recursion and glob patterns --------------------------------------------

// fileTree writes each name (forward-slash relative) with the given content
// under a fresh temp dir and returns the root.
func fileTree(t *testing.T, content string, names ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range names {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// inDir runs fn with the working directory set to dir, so patterns relative to
// the current directory can be tested the way a user would type them.
func inDir(t *testing.T, dir string, fn func()) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(prev); err != nil {
			t.Fatal(err)
		}
	}()
	fn()
}

func TestDirectoryIsNotRecursiveWithoutFlag(t *testing.T) {
	root := fileTree(t, "x", "top.md", "sub/nested.md")

	shallow, err := expandTargets([]string{root}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(shallow) != 1 || filepath.Base(shallow[0]) != "top.md" {
		t.Errorf("without -r got %v, want only top.md", shallow)
	}

	deep, err := expandTargets([]string{root}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(deep) != 2 {
		t.Errorf("with -r got %v, want both files", deep)
	}
}

func TestGlobPatternRecursive(t *testing.T) {
	root := fileTree(t, "package x\n",
		"a.go", "sub/b.go", "sub/deep/c.go", "sub/notes.md", "vendor/skip.go")

	inDir(t, root, func() {
		got, err := expandTargets([]string{"*.go"}, true)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, p := range got {
			names = append(names, filepath.ToSlash(p))
		}
		slices.Sort(names)
		want := []string{"a.go", "sub/b.go", "sub/deep/c.go"}
		if !slices.Equal(names, want) {
			t.Errorf("got %v, want %v", names, want)
		}
	})
}

func TestGlobPatternNonRecursive(t *testing.T) {
	root := fileTree(t, "package x\n", "a.go", "sub/b.go")

	inDir(t, root, func() {
		got, err := expandTargets([]string{"*.go"}, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || filepath.Base(got[0]) != "a.go" {
			t.Errorf("got %v, want only a.go", got)
		}
	})
}

func TestDoubleStarForcesRecursion(t *testing.T) {
	root := fileTree(t, "x\n", "a.md", "sub/deep/b.md")

	inDir(t, root, func() {
		// No -r flag, but ** means recursive anyway.
		got, err := expandTargets([]string{"**/*.md"}, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Errorf("got %v, want both files", got)
		}
	})
}

func TestGlobIgnoresExtensionWhitelist(t *testing.T) {
	// An explicit pattern is an instruction; .xyz is not in TextExtensions but
	// the user asked for it by name.
	root := fileTree(t, "text\n", "sub/thing.xyz")

	inDir(t, root, func() {
		got, err := expandTargets([]string{"*.xyz"}, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Errorf("got %v, want the .xyz file", got)
		}
	})
}

func TestUnmatchedPatternIsAnError(t *testing.T) {
	root := fileTree(t, "x", "a.md")
	inDir(t, root, func() {
		if _, err := expandTargets([]string{"*.nope"}, true); !errors.Is(err, errNoFiles) {
			t.Errorf("err = %v, want errNoFiles", err)
		}
	})
}

func TestPartiallyUnmatchedPatternStillRuns(t *testing.T) {
	// One good pattern and one bad one: the run proceeds on what matched.
	root := fileTree(t, "x", "a.md")
	inDir(t, root, func() {
		got, err := expandTargets([]string{"*.md", "*.nope"}, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Errorf("got %v, want the one match", got)
		}
	})
}

func TestTargetsAreDeduplicated(t *testing.T) {
	root := fileTree(t, "x", "a.md")
	inDir(t, root, func() {
		got, err := expandTargets([]string{"*.md", "a.md", "*.md"}, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Errorf("got %v, want one entry after de-duplication", got)
		}
	})
}

func TestWildcardInIntermediateComponentIsRejectedWhenRecursive(t *testing.T) {
	root := fileTree(t, "x", "a/b.go")
	inDir(t, root, func() {
		_, err := expandTargets([]string{"*/x/*.go"}, true)
		if err == nil || !strings.Contains(err.Error(), "final path component") {
			t.Errorf("err = %v, want a clear explanation", err)
		}
	})
}

func TestNoArgumentsMeansStdin(t *testing.T) {
	got, err := expandTargets(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "-" {
		t.Errorf("got %v, want [-]", got)
	}
}

func TestInspectRecursiveGlobEndToEnd(t *testing.T) {
	root := fileTree(t, "he​llo\n", "a.go", "sub/b.go")
	inDir(t, root, func() {
		code, err := runInspect([]string{"-json", "-r", "*.go"})
		if err != nil {
			t.Fatal(err)
		}
		if code != 1 {
			t.Errorf("code = %d, want 1 (both files carry a mark)", code)
		}
	})
}

func TestMultiFileRunRequiresExplicitDestination(t *testing.T) {
	// Regression: "wmr clean -r ." used to scatter a .cleaned.* sibling next to
	// every file, which in a Go package is a duplicate declaration that breaks
	// the build.
	root := fileTree(t, "he​llo\n", "a.go", "sub/b.go")

	code, err := runClean([]string{"-r", root})
	if code != 2 || err == nil {
		t.Fatalf("code=%d err=%v, want a refusal", code, err)
	}
	if !strings.Contains(err.Error(), "-in-place") {
		t.Errorf("error should name the way forward: %v", err)
	}

	// Nothing was written.
	if entries, _ := filepath.Glob(filepath.Join(root, "*.cleaned.*")); len(entries) > 0 {
		t.Errorf("files were written despite the refusal: %v", entries)
	}

	// The explicit forms are all still accepted.
	for _, args := range [][]string{
		{"-r", "-dry-run", root},
		{"-r", "-in-place", root},
	} {
		if code, err := runClean(args); err != nil || code != 0 {
			t.Errorf("runClean(%v): code=%d err=%v", args, code, err)
		}
	}
}

func TestSingleFileStillWritesCleanedSibling(t *testing.T) {
	// The convenient default survives for the one-file case.
	src := write(t, "notes.txt", "he​llo\n")
	if code, err := runClean([]string{src}); err != nil || code != 0 {
		t.Fatalf("code=%d err=%v", code, err)
	}
	dest := strings.TrimSuffix(src, ".txt") + ".cleaned.txt"
	if got := read(t, dest); got != "hello\n" {
		t.Errorf("cleaned = %q", got)
	}
}

func TestInPlaceFlagSpellings(t *testing.T) {
	// Both spellings, with one dash or two. The hyphen is easy to drop and
	// "flag provided but not defined" is a poor answer to a near-miss.
	for _, spelling := range []string{"-in-place", "--in-place", "-inplace", "--inplace"} {
		t.Run(spelling, func(t *testing.T) {
			src := write(t, "notes.txt", "he​llo\n")
			if code, err := runClean([]string{spelling, src}); err != nil || code != 0 {
				t.Fatalf("code=%d err=%v", code, err)
			}
			if got := read(t, src); got != "hello\n" {
				t.Errorf("file = %q, want the cleaned text in place", got)
			}
			if _, err := os.Stat(src + ".bak"); err != nil {
				t.Errorf("no backup written: %v", err)
			}
		})
	}
}

func TestNoTargetMatchedIsExitTwo(t *testing.T) {
	// Documented contract: matching nothing is an error, not a quiet success.
	root := fileTree(t, "x", "a.md")
	inDir(t, root, func() {
		for _, cmd := range []func([]string) (int, error){runInspect, runClean} {
			code, err := cmd([]string{"-r", "*.nomatch"})
			if code != 2 || !errors.Is(err, errNoFiles) {
				t.Errorf("code=%d err=%v, want 2 / errNoFiles", code, err)
			}
		}
	})
}
