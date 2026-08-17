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

// fileReport is the JSON shape of one inspected file.
type fileReport struct {
	Path     string          `json:"path"`
	Report   marks.Report    `json:"unicode"`
	Metadata *docmeta.Result `json:"metadata,omitempty"`
	Lines    *lines.Result   `json:"lines,omitempty"`
}

// Suspicious reports whether anything at all was found in this file.
func (f fileReport) Suspicious() bool {
	return f.Report.SuspiciousTotal > 0 ||
		(f.Metadata != nil && f.Metadata.HasAI) ||
		(f.Lines != nil && f.Lines.Removed > 0)
}

func runInspect(args []string) (int, error) {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "emit a JSON report instead of human-readable text")
	aggressive := fs.Bool("aggressive", false, "also flag Latin confusable / fullwidth lookalikes")
	glue := fs.Bool("strip-emoji-glue", false, "paranoid: also flag load-bearing invisibles (emoji glue, script joiners, flag tags, same-script fillers)")
	noMeta := fs.Bool("no-metadata", false, "skip Markdown frontmatter / HTML meta inspection")
	filterOpts := registerFilterFlags(fs)
	verbose := fs.Bool("verbose", false, "name every file, including the clean ones")
	forceText := fs.Bool("force-text", false, "scan even when the input looks like a binary container")
	recursive := fs.Bool("r", false, "search directories and match glob patterns recursively")
	fs.BoolVar(recursive, "recursive", false, "alias for -r")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: wmr inspect [flags] [target ...]\n\n%s\nFlags:\n", targetHelp)
		fs.PrintDefaults()
	}
	if err := parseAnywhere(fs, args); err != nil {
		return 2, err
	}

	// inspect never writes, so the line filter is pure preview here: it
	// reports which lines a clean with the same flags would delete.
	filter, err := filterOpts.build()
	if err != nil {
		return 2, err
	}

	targets, err := expandTargets(fs.Args(), *recursive)
	if err != nil {
		return 2, err
	}

	opt := marks.InspectOptions{Aggressive: *aggressive, StripEmojiGlue: *glue}
	reports := make([]fileReport, 0, len(targets))

	for _, path := range targets {
		text, err := textio.ReadText(path, *forceText)
		if err != nil {
			return 2, err
		}
		fr := fileReport{Path: displayName(path), Report: marks.Inspect(text, opt)}
		if !*noMeta {
			if format := docmeta.Format(ext(path)); format != "" {
				res := docmeta.Inspect(format, text)
				fr.Metadata = &res
			}
		}
		if filter.Active() {
			if _, res := filter.Apply(text); res.Removed > 0 {
				fr.Lines = &res
			}
		}
		reports = append(reports, fr)
	}

	if *asJSON {
		if err := emitJSON(reports, len(targets) == 1); err != nil {
			return 2, err
		}
	} else {
		printHuman(reports, *verbose)
	}

	flagged := 0
	for _, r := range reports {
		if r.Suspicious() {
			flagged++
		}
	}
	if len(reports) > 1 && !*asJSON {
		fmt.Fprintf(os.Stderr, "Processed %d files, flagged %d.\n", len(reports), flagged)
	}
	if flagged > 0 {
		return 1, nil
	}
	return 0, nil
}

// emitJSON writes one object for a single file and an array for several, so
// piping a single file into jq stays simple.
func emitJSON(reports []fileReport, single bool) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if single && len(reports) == 1 {
		return enc.Encode(reports[0])
	}
	return enc.Encode(reports)
}

func printHuman(reports []fileReport, verbose bool) {
	multi := len(reports) > 1
	first := true

	for _, r := range reports {
		// Across a whole tree, the clean files are noise; a single explicit
		// file is always reported, since the user asked about that one.
		if multi && !verbose && !r.Suspicious() {
			continue
		}
		if !first {
			fmt.Println()
		}
		first = false

		if multi {
			fmt.Printf("== %s ==\n", r.Path)
		}
		fmt.Println(r.Report.Human())

		if r.Metadata != nil && len(r.Metadata.Findings) > 0 {
			fmt.Printf("\n%s metadata:\n", r.Metadata.Format)
			for _, f := range r.Metadata.Findings {
				fmt.Printf("  %s\n", f)
			}
		}
		if r.Lines != nil {
			fmt.Printf("\nattribution lines (%d would be removed):\n", r.Lines.Removed)
			for _, m := range sampleMatches(*r.Lines, verbose) {
				fmt.Printf("  line %d [%s]: %s\n", m.Line, m.Pattern, truncate(m.Text, 100))
			}
		}
	}
}
