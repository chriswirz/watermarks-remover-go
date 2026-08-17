package main

import (
	"flag"
	"strings"
)

// parseAnywhere parses flags that appear after positional arguments, which
// the standard flag package stops at. "wmr clean notes.md -o out.md" is the
// order people actually type, and silently treating "-o" as a filename there
// produced a confusing "no such file" instead of doing the obvious thing.
//
// It reorders args so every flag precedes every operand, then parses. A bare
// "--" ends flag parsing, as usual: everything after it is an operand.
func parseAnywhere(fs *flag.FlagSet, args []string) error {
	var flags, operands []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			operands = append(operands, arg)
			continue
		}

		flags = append(flags, arg)
		// A non-boolean flag written without "=" takes the next argument as
		// its value, so that argument is not an operand.
		if !strings.Contains(arg, "=") && takesValue(fs, arg) && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}

	// The "--" terminator goes back in unconditionally: an operand that
	// begins with a dash (a file literally named "-weird.md") must not be
	// re-read as a flag now that it sits after the real ones.
	return fs.Parse(append(append(flags, "--"), operands...))
}

// takesValue reports whether the named flag consumes a following argument.
// Boolean flags do not: "-json out.md" means the flag and then a file.
func takesValue(fs *flag.FlagSet, arg string) bool {
	name := strings.TrimLeft(arg, "-")
	f := fs.Lookup(name)
	if f == nil {
		return false // unknown flag; let Parse produce the error
	}
	boolFlag, ok := f.Value.(interface{ IsBoolFlag() bool })
	return !ok || !boolFlag.IsBoolFlag()
}
