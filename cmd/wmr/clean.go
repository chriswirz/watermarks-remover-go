package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/chriswirz/watermarks-remover-go/internal/docmeta"
	"github.com/chriswirz/watermarks-remover-go/internal/lines"
	"github.com/chriswirz/watermarks-remover-go/internal/marks"
	"github.com/chriswirz/watermarks-remover-go/internal/textio"
)

func runClean(args []string) (int, error) {
	fs := flag.NewFlagSet("clean", flag.ExitOnError)
	output := fs.String("o", "", "output path (default: stdout for stdin, else *.cleaned.*)")
	inPlace := fs.Bool("in-place", false, "overwrite each input file, keeping a .bak copy")
	// Both spellings are in circulation and the hyphen is easy to drop.
	// Go's flag package accepts one or two leading dashes for either.
	fs.BoolVar(inPlace, "inplace", false, "alias for -in-place")
	nfkc := fs.Bool("nfkc", false, "apply Unicode NFKC normalization after the scrub")
	homoglyphs := fs.Bool("aggressive-homoglyphs", false, "map Cyrillic/fullwidth Latin confusables to ASCII")
	keepSpaces := fs.Bool("keep-spaces", false, "do not rewrite exotic spaces to U+0020")
	glue := fs.Bool("strip-emoji-glue", false, "paranoid: also strip load-bearing invisibles; this can visibly alter emoji and non-Latin scripts")
	stripBidi := fs.Bool("strip-bidi", false, "also strip legitimate RTL/LTR directional marks and isolates")
	visible := fs.String("visible", "", `force visible characters only: "utf8" keeps printable Unicode, "ascii" folds to printable ASCII (both keep space, tab, CR, LF)`)
	noMeta := fs.Bool("no-metadata", false, "skip Markdown frontmatter / HTML meta cleaning")
	filterOpts := registerFilterFlags(fs)
	dryRun := fs.Bool("dry-run", false, "report what would change without writing anything")
	verbose := fs.Bool("verbose", false, "name every file, including the unchanged ones")
	stats := fs.Bool("stats", false, "print the full stats breakdown to stderr")
	asJSON := fs.Bool("json", false, "print stats as JSON to stderr")
	forceText := fs.Bool("force-text", false, "clean even when the input looks binary (this rewrites the bytes and will corrupt the file)")
	recursive := fs.Bool("r", false, "search directories and match glob patterns recursively")
	fs.BoolVar(recursive, "recursive", false, "alias for -r")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: wmr clean [flags] [target ...]\n\n%s\nFlags:\n", targetHelp)
		fs.PrintDefaults()
	}
	if err := parseAnywhere(fs, args); err != nil {
		return 2, err
	}

	mode := marks.VisibleMode(*visible)
	if !mode.Valid() {
		return 2, fmt.Errorf("-visible: unknown mode %q; use \"utf8\" or \"ascii\"", *visible)
	}
	filter, err := filterOpts.build()
	if err != nil {
		return 2, err
	}

	targets, err := expandTargets(fs.Args(), *recursive)
	if err != nil {
		return 2, err
	}
	if *output != "" {
		if len(targets) > 1 {
			return 2, fmt.Errorf("-o takes a single input file; got %d", len(targets))
		}
		if *inPlace {
			return 2, fmt.Errorf("-o and -in-place are mutually exclusive")
		}
	}

	// Writing "*.cleaned.*" siblings is a fine default for one named file, and
	// a bad one for a whole tree: it litters the source with copies, and in a
	// compiled language each copy is a duplicate of its neighbour that breaks
	// the build. Make the destination explicit once more than one file is in
	// play.
	if len(targets) > 1 && *output == "" && !*inPlace && !*dryRun {
		return 2, fmt.Errorf(
			"refusing to write %d separate .cleaned.* files into your tree; "+
				"pass -in-place to rewrite the originals (a .bak is kept), or "+
				"-dry-run to see what would change", len(targets))
	}

	opt := marks.Options{
		NFKC:                 *nfkc,
		AggressiveHomoglyphs: *homoglyphs,
		KeepSpaces:           *keepSpaces,
		StripEmojiGlue:       *glue,
		StripBidi:            *stripBidi,
		Visible:              mode,
	}
	flags := cleanFlags{
		inPlace: *inPlace,
		noMeta:  *noMeta,
		stats:   *stats,
		asJSON:  *asJSON,
		force:   *forceText,
		dryRun:  *dryRun,
		verbose: *verbose,
		multi:   len(targets) > 1,
		filter:  filter,
	}

	modified := 0
	for _, path := range targets {
		changed, err := cleanOne(path, *output, opt, flags)
		if err != nil {
			return 2, err
		}
		if changed {
			modified++
		}
	}

	// The summary only earns its line when a whole tree was processed; for a
	// single file the per-file stats already said everything.
	if len(targets) > 1 {
		verb := "modified"
		if *dryRun {
			verb = "would modify"
		}
		fmt.Fprintf(os.Stderr, "Processed %d files, %s %d.\n", len(targets), verb, modified)
	}
	return 0, nil
}

type cleanFlags struct {
	inPlace, noMeta, stats, asJSON, force, dryRun, verbose, multi bool
	filter                                                        lines.Filter
}

func cleanOne(path, output string, opt marks.Options, f cleanFlags) (bool, error) {
	isStdin := path == "-" || path == ""
	if f.inPlace && isStdin {
		return false, fmt.Errorf("-in-place needs a file path, not stdin")
	}

	text, err := textio.ReadText(path, f.force)
	if err != nil {
		return false, err
	}

	var metaActions []string
	if !f.noMeta {
		if format := docmeta.Format(ext(path)); format != "" {
			text, metaActions = docmeta.Clean(format, text)
		}
	}

	// Line removal runs before the rune scrub: it deletes whole lines, so
	// scrubbing them first would be wasted work and would inflate the stats
	// with carriers that never reach the output.
	text, lineResult := f.filter.Apply(text)

	cleaned, st := marks.Clean(text, opt)
	changed := st.Changed() || lineResult.Removed > 0 || len(metaActions) > 0

	if f.dryRun {
		reportDryRun(path, st, lineResult, metaActions, f)
		return changed, nil
	}
	if !changed && !f.verbose && f.multi {
		// In a tree walk, silence on the untouched files is the useful default.
		return false, nil
	}

	// Back up before writing, so an unwritable destination cannot leave the
	// original gone. The backup is a full copy of the untouched input.
	dest := output
	switch {
	case f.inPlace:
		if _, err := textio.Backup(path); err != nil {
			return false, err
		}
		dest = path
	case dest == "" && !isStdin:
		dest = textio.CleanedPath(path)
	}

	if err := textio.WriteText(dest, cleaned); err != nil {
		return false, err
	}
	return changed, reportClean(path, dest, st, lineResult, metaActions, f)
}

func reportDryRun(path string, st marks.Stats, lr lines.Result, metaActions []string, f cleanFlags) {
	if !st.Changed() && lr.Removed == 0 && len(metaActions) == 0 {
		if f.verbose {
			fmt.Fprintf(os.Stderr, "unchanged: %s\n", displayName(path))
		}
		return
	}

	fmt.Fprintf(os.Stderr, "would change %s: %s", displayName(path), st.Human(false))
	if lr.Removed > 0 {
		fmt.Fprintf(os.Stderr, " lines_removed=%d", lr.Removed)
	}
	fmt.Fprintln(os.Stderr)

	for _, m := range sampleMatches(lr, f.verbose) {
		fmt.Fprintf(os.Stderr, "    - line %d [%s]: %s\n", m.Line, m.Pattern, truncate(m.Text, 100))
	}
	if f.verbose {
		for _, a := range metaActions {
			fmt.Fprintln(os.Stderr, "    metadata: "+a)
		}
	}
}

// sampleMatches limits dry-run detail to a readable number of lines unless
// -verbose asks for everything the filter recorded.
func sampleMatches(lr lines.Result, verbose bool) []lines.Match {
	const sample = 5
	if verbose || len(lr.Matches) <= sample {
		return lr.Matches
	}
	return lr.Matches[:sample]
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func reportClean(path, dest string, st marks.Stats, lr lines.Result, metaActions []string, f cleanFlags) error {
	if f.asJSON {
		enc := json.NewEncoder(os.Stderr)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Path            string       `json:"path"`
			Output          string       `json:"output"`
			Stats           marks.Stats  `json:"stats"`
			Lines           lines.Result `json:"lines"`
			MetadataActions []string     `json:"metadata_actions,omitempty"`
		}{displayName(path), displayName(dest), st, lr, metaActions})
	}

	prefix := ""
	if f.multi {
		prefix = displayName(path) + ": "
	}
	summary := st.Human(f.stats)
	if lr.Removed > 0 {
		summary += fmt.Sprintf(" lines_removed=%d", lr.Removed)
	}
	fmt.Fprintln(os.Stderr, prefix+summary)

	if f.stats {
		for _, m := range sampleMatches(lr, f.verbose) {
			fmt.Fprintf(os.Stderr, "    - line %d [%s]: %s\n", m.Line, m.Pattern, truncate(m.Text, 100))
		}
		for _, a := range metaActions {
			fmt.Fprintln(os.Stderr, "  metadata: "+a)
		}
	}
	return nil
}
