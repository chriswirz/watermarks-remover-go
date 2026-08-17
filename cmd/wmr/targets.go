package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/chriswirz/watermarks-remover-go/internal/textio"
)

// targetHelp explains what the positional arguments accept. Both commands show
// it, because the answer must not differ between them.
const targetHelp = `Targets may be files, directories, or glob patterns:

  wmr inspect notes.md            a single file
  wmr inspect ./docs              files directly in a directory
  wmr inspect -r ./docs           ... and everything beneath it
  wmr inspect -r "*.go"           every .go file in this tree
  wmr inspect "**/*.md"           ** is recursive whatever -r says

With no target at all, reads stdin.

Quote patterns on POSIX shells: bash expands *.go itself, before wmr sees it,
and its expansion is never recursive. cmd and PowerShell pass patterns through
untouched, so quoting there is optional.
`

// errNoFiles is returned when the arguments resolve to nothing to do. It is an
// error rather than a silent success: a command that matched no files did not
// do what the user asked, and a script should notice.
var errNoFiles = errors.New("no files matched")

// expandTargets resolves the positional arguments to a list of files. With no
// arguments at all it falls back to stdin, matching the single-file usage.
func expandTargets(args []string, recursive bool) ([]string, error) {
	if len(args) == 0 {
		return []string{"-"}, nil
	}

	exp, err := textio.Expand(args, recursive)
	if err != nil {
		return nil, err
	}

	// A pattern matching nothing is usually a typo, and worth saying out loud
	// even when other patterns did match, but it is only fatal if nothing
	// matched at all.
	for _, p := range exp.Unmatched {
		fmt.Fprintf(os.Stderr, "warning: no files matched %q\n", p)
	}
	if len(exp.Files) == 0 {
		return nil, errNoFiles
	}
	return exp.Files, nil
}
