package textio

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// tree builds a directory tree from a path->content map and returns its root.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
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

// relative returns walk results as forward-slash paths relative to root, so
// assertions read the same on every platform.
func relative(t *testing.T, root string, paths []string) []string {
	t.Helper()
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		rel, err := filepath.Rel(root, p)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, filepath.ToSlash(rel))
	}
	slices.Sort(out)
	return out
}

func TestWalkCollectsTextFiles(t *testing.T) {
	root := tree(t, map[string]string{
		"README.md":      "# hi",
		"main.go":        "package main",
		"docs/notes.txt": "notes",
		"docs/deep/a.py": "print()",
		"image.png":      "\x89PNG",
		"archive.zip":    "PK\x03\x04",
		"noextension":    "text",
		"styles.css":     "body{}",
	})

	got := relative(t, root, mustWalk(t, root))
	want := []string{"README.md", "docs/deep/a.py", "docs/notes.txt", "main.go", "styles.css"}
	if !slices.Equal(got, want) {
		t.Errorf("walk = %v, want %v", got, want)
	}
}

func TestWalkSkipsVendorAndVCS(t *testing.T) {
	root := tree(t, map[string]string{
		"main.go":                   "package main",
		".git/config":               "text",
		".git/hooks/pre-commit.sh":  "#!/bin/sh",
		"node_modules/pkg/index.js": "module.exports={}",
		"vendor/lib/thing.go":       "package lib",
		"__pycache__/x.py":          "cached",
		"dist/out.js":               "bundled",
		"src/app.js":                "real source",
	})

	got := relative(t, root, mustWalk(t, root))
	want := []string{"main.go", "src/app.js"}
	if !slices.Equal(got, want) {
		t.Errorf("walk = %v, want %v", got, want)
	}
}

func TestWalkDoesNotSkipRootEvenWhenNamed(t *testing.T) {
	// Pointing the tool at a directory called "build" is an explicit
	// instruction; only nested matches should be pruned.
	parent := t.TempDir()
	root := filepath.Join(parent, "build")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := relative(t, root, mustWalk(t, root))
	if !slices.Equal(got, []string{"a.go"}) {
		t.Errorf("walk = %v, want [a.go]", got)
	}
}

func TestWalkOnFileReturnsItRegardlessOfExtension(t *testing.T) {
	// An explicit file argument is an explicit instruction; the extension
	// whitelist only governs what a directory walk picks up.
	root := tree(t, map[string]string{"odd.xyz": "some text"})
	path := filepath.Join(root, "odd.xyz")

	got := mustWalk(t, path)
	if len(got) != 1 || got[0] != path {
		t.Errorf("walk = %v, want [%s]", got, path)
	}
}

func TestWalkMissingPath(t *testing.T) {
	if _, err := Walk(filepath.Join(t.TempDir(), "nope"), true); err == nil {
		t.Error("expected an error for a missing path")
	}
}

func TestIsTextFile(t *testing.T) {
	for _, yes := range []string{"a.go", "a.MD", "a.Py", "dir/b.yaml", "x.json"} {
		if !IsTextFile(yes) {
			t.Errorf("IsTextFile(%q) = false", yes)
		}
	}
	for _, no := range []string{"a.png", "a.zip", "a.exe", "noext", "a.docx"} {
		if IsTextFile(no) {
			t.Errorf("IsTextFile(%q) = true", no)
		}
	}
}

func TestIsDir(t *testing.T) {
	root := tree(t, map[string]string{"f.txt": "x"})
	if !IsDir(root) {
		t.Error("IsDir(dir) = false")
	}
	if IsDir(filepath.Join(root, "f.txt")) {
		t.Error("IsDir(file) = true")
	}
	for _, s := range []string{"", "-", filepath.Join(root, "missing")} {
		if IsDir(s) {
			t.Errorf("IsDir(%q) = true", s)
		}
	}
}

func mustWalk(t *testing.T, root string) []string {
	t.Helper()
	got, err := Walk(root, true)
	if err != nil {
		t.Fatalf("Walk(%s): %v", root, err)
	}
	return got
}

func TestWalkResultsAreUsable(t *testing.T) {
	// The paths a walk returns must be readable as-is, not relative to some
	// other directory.
	root := tree(t, map[string]string{"a/b/c.md": "content"})
	for _, p := range mustWalk(t, root) {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("cannot read walked path %q: %v", p, err)
		}
		if !strings.Contains(string(data), "content") {
			t.Errorf("unexpected content at %q", p)
		}
	}
}

func TestIsToolOutput(t *testing.T) {
	for _, yes := range []string{
		"x.cleaned.go", "notes.cleaned.md", "dir/a.cleaned.txt",
		"notes.md.bak", "x.bak",
	} {
		if !IsToolOutput(yes) {
			t.Errorf("IsToolOutput(%q) = false", yes)
		}
	}
	for _, no := range []string{
		"x.go", "cleaned.go", "a.cleanedup.go", "backup.md", "x.bakery",
	} {
		if IsToolOutput(no) {
			t.Errorf("IsToolOutput(%q) = true", no)
		}
	}
}

func TestSearchesSkipToolOutput(t *testing.T) {
	// Re-running the tool over a tree it already cleaned must not pick up its
	// own output; in a Go package those siblings are duplicate declarations.
	root := tree(t, map[string]string{
		"a.go":             "package a",
		"a.cleaned.go":     "package a",
		"a.go.bak":         "package a",
		"sub/b.md":         "text",
		"sub/b.cleaned.md": "text",
	})

	got := relative(t, root, mustWalk(t, root))
	want := []string{"a.go", "sub/b.md"}
	if !slices.Equal(got, want) {
		t.Errorf("walk = %v, want %v", got, want)
	}

	exp, err := Expand([]string{filepath.Join(root, "**/*.go")}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range exp.Files {
		if IsToolOutput(f) {
			t.Errorf("glob picked up tool output: %s", f)
		}
	}
}

func TestExplicitToolOutputPathIsHonored(t *testing.T) {
	// The skip governs searches only. Naming the file directly still works.
	root := tree(t, map[string]string{"a.cleaned.go": "package a"})
	path := filepath.Join(root, "a.cleaned.go")

	exp, err := Expand([]string{path}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(exp.Files) != 1 || exp.Files[0] != path {
		t.Errorf("Expand = %v, want the explicitly named file", exp.Files)
	}
}
