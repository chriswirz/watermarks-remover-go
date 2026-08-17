package textio

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// TextExtensions are the file types a directory search will pick up. Searching
// a directory needs a whitelist rather than the content sniff used for
// explicit arguments: descending a tree and guessing at every file is how a
// tool ends up rewriting something it was never pointed at.
//
// An explicit path or a glob pattern bypasses this entirely: naming a file is
// an instruction, and the binary guard still applies when it is read.
var TextExtensions = map[string]bool{
	// Source
	".go": true, ".py": true, ".js": true, ".jsx": true, ".ts": true,
	".tsx": true, ".java": true, ".c": true, ".cpp": true, ".cc": true,
	".h": true, ".hpp": true, ".rs": true, ".rb": true, ".php": true,
	".lua": true, ".r": true, ".swift": true, ".kt": true, ".kts": true,
	".cs": true, ".scala": true, ".pl": true, ".sql": true,

	// Shell
	".sh": true, ".bash": true, ".zsh": true, ".fish": true, ".ps1": true,

	// Markup and prose
	".md": true, ".markdown": true, ".mdx": true, ".txt": true, ".text": true,
	".rst": true, ".adoc": true, ".tex": true,
	".html": true, ".htm": true, ".xml": true, ".svg": true, ".css": true,
	".scss": true, ".less": true,

	// Data and config
	".json": true, ".yaml": true, ".yml": true, ".toml": true, ".ini": true,
	".cfg": true, ".conf": true, ".env": true, ".csv": true, ".tsv": true,
}

// skipDirs are directories a search never descends into: version-control
// metadata and dependency or build trees, where the files are not the user's
// to rewrite and the volume is enormous. These are skipped for glob patterns
// too, since `-r *.go` in a Go repo should not walk into vendor/.
var skipDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true,
	"node_modules": true, "vendor": true, "target": true,
	".venv": true, "venv": true, "__pycache__": true,
	"dist": true, "build": true, ".idea": true, ".vscode": true,
}

// IsTextFile reports whether a path's extension is one a directory search collects.
func IsTextFile(path string) bool {
	return TextExtensions[strings.ToLower(filepath.Ext(path))]
}

// IsToolOutput reports whether a path looks like this tool's own output: a
// "*.cleaned.*" sibling or a "*.bak" backup.
//
// Searches skip these. Re-cleaning an already-cleaned file is pointless, and
// in a source tree a file like visible_test.cleaned.go is a compilable
// duplicate of its neighbour that breaks the build. An explicitly named path
// is still honored; this only governs what a search picks up on its own.
func IsToolOutput(path string) bool {
	base := filepath.Base(path)
	if strings.HasSuffix(base, ".bak") {
		return true
	}
	ext := filepath.Ext(base)
	return strings.HasSuffix(strings.TrimSuffix(base, ext), ".cleaned")
}

// HasGlobMeta reports whether an argument looks like a glob pattern rather
// than a plain path.
func HasGlobMeta(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

// Expansion is the result of turning command-line arguments into a file list.
type Expansion struct {
	// Files are the paths to process, de-duplicated, in argument order.
	Files []string
	// Unmatched are the glob patterns that matched nothing. These are worth
	// reporting, since a pattern that finds no files is usually a typo, but they
	// are not fatal on their own, since one empty pattern among several
	// should not abort the run.
	Unmatched []string
}

// Expand turns command-line arguments into the list of files to process.
//
// Each argument is one of:
//
//   - "-", passed through as stdin;
//   - a glob pattern (contains *, ? or [), matched against file names;
//   - a directory, searched for known text extensions;
//   - anything else, taken literally as a file path.
//
// recursive controls the depth of both directory searches and glob matching.
// A pattern containing "**" is always recursive regardless of the flag.
func Expand(args []string, recursive bool) (Expansion, error) {
	var exp Expansion
	seen := make(map[string]bool)

	add := func(paths ...string) {
		for _, p := range paths {
			if seen[p] {
				continue
			}
			seen[p] = true
			exp.Files = append(exp.Files, p)
		}
	}

	for _, arg := range args {
		switch {
		case arg == "-" || arg == "":
			add(arg)

		case HasGlobMeta(arg):
			matches, err := globFiles(arg, recursive)
			if err != nil {
				return exp, err
			}
			if len(matches) == 0 {
				exp.Unmatched = append(exp.Unmatched, arg)
				continue
			}
			add(matches...)

		case IsDir(arg):
			found, err := searchDir(arg, recursive, func(p string) bool {
				return IsTextFile(p) && !IsToolOutput(p)
			})
			if err != nil {
				return exp, err
			}
			add(found...)

		default:
			add(arg)
		}
	}
	return exp, nil
}

// globFiles resolves a glob pattern to a list of files.
//
// Non-recursively this is filepath.Glob, so patterns with wildcards in
// intermediate components ("src/*/main.go") work as usual. Recursively, the
// directory part fixes where the search starts and the final component is
// matched against every file name beneath it.
func globFiles(pattern string, recursive bool) ([]string, error) {
	dir, base, forced := splitPattern(pattern)
	if !recursive && !forced {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("bad pattern %q: %w", pattern, err)
		}
		out := make([]string, 0, len(matches))
		for _, m := range filesOnly(matches) {
			if !IsToolOutput(m) {
				out = append(out, m)
			}
		}
		return out, nil
	}

	if HasGlobMeta(dir) {
		return nil, fmt.Errorf(
			"pattern %q: a recursive search only supports wildcards in the final path component; "+
				"use %q or pass the directory as an argument",
			pattern, "**/"+base)
	}
	// Validate the pattern once here, so a malformed one is an error rather
	// than silently matching nothing on every file in the tree.
	if _, err := filepath.Match(base, "probe"); err != nil {
		return nil, fmt.Errorf("bad pattern %q: %w", pattern, err)
	}

	return searchDir(dir, true, func(path string) bool {
		if IsToolOutput(path) {
			return false
		}
		ok, _ := filepath.Match(base, filepath.Base(path))
		return ok
	})
}

// splitPattern separates a glob into the directory to search and the name
// pattern to match. A "**" component means "search recursively from here" and
// forces recursion on regardless of the -r flag.
func splitPattern(pattern string) (dir, base string, forced bool) {
	normalized := filepath.ToSlash(pattern)

	if idx := strings.Index(normalized, "**"); idx >= 0 {
		forced = true
		dir = strings.TrimSuffix(normalized[:idx], "/")
		base = strings.TrimPrefix(normalized[idx+2:], "/")
		if base == "" {
			base = "*"
		}
		if dir == "" {
			dir = "."
		}
		return filepath.FromSlash(dir), base, true
	}

	dir, base = filepath.Split(pattern)
	if dir == "" {
		dir = "."
	}
	return filepath.Clean(dir), base, false
}

// filesOnly drops directories from a glob result: a pattern is asking for
// files to process, and a matched directory would otherwise be read as one.
func filesOnly(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if !IsDir(p) {
			out = append(out, p)
		}
	}
	return out
}

// searchDir collects files under root that accept approves. When recursive is
// false only the direct children are considered.
func searchDir(root string, recursive bool, accept func(path string) bool) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == root {
				return nil // never skip the root, even if it is named "build"
			}
			if !recursive || skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if accept(path) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// Walk returns the text files under root. A root that is itself a file is
// returned as-is without an extension check.
func Walk(root string, recursive bool) ([]string, error) {
	info, err := statPath(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{root}, nil
	}
	return searchDir(root, recursive, func(p string) bool {
		return IsTextFile(p) && !IsToolOutput(p)
	})
}
