package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/chriswirz/watermarks-remover-go/internal/lines"
)

// filterFlags are the line-removal options shared by inspect and clean, so the
// two commands cannot drift on how a pattern set is assembled.
type filterFlags struct {
	attribution *bool
	patterns    *string
	patternFile *string
	comments    *bool
}

func registerFilterFlags(fs *flag.FlagSet) filterFlags {
	return filterFlags{
		attribution: fs.Bool("attribution", false,
			"remove whole lines matching the built-in AI attribution patterns (destructive; preview with -dry-run)"),
		patterns: fs.String("patterns", "",
			"comma-separated regexps; whole matching lines are removed. Replaces the built-in set rather than adding to it"),
		patternFile: fs.String("pattern-file", "",
			"file with one regexp per line (blank lines and # comments skipped), added to whichever set is in use"),
		comments: fs.Bool("strip-comments", false,
			"remove every single-line comment, regardless of content (breaks build tags, shebangs and pragmas)"),
	}
}

// build assembles the line filter from the flags.
//
// The precedence mirrors the original tool: -patterns replaces the built-in
// set, -pattern-file adds to whatever is in use, and the built-in set is only
// loaded when -attribution asks for it. Line deletion never happens by
// accident: some flag has to request it.
func (f filterFlags) build() (lines.Filter, error) {
	var filter lines.Filter
	var specs []string

	switch {
	case *f.patterns != "":
		specs = splitPatterns(*f.patterns)
	case *f.attribution:
		specs = lines.DefaultPatterns()
	}

	if *f.patternFile != "" {
		extra, err := lines.LoadPatternFile(*f.patternFile)
		if err != nil {
			return filter, fmt.Errorf("-pattern-file: %w", err)
		}
		specs = append(specs, extra...)
	}

	compiled, err := lines.Compile(specs)
	if err != nil {
		return filter, err
	}
	filter.Patterns = compiled
	filter.StripComments = *f.comments
	return filter, nil
}

// splitPatterns splits a comma-separated flag value, dropping empty entries.
// A regexp containing a comma has to come from -pattern-file instead; that
// limitation is inherited from the original tool's flag format.
func splitPatterns(value string) []string {
	var out []string
	for _, p := range strings.Split(value, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
