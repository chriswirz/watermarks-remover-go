// Command wmr inspects and removes AI provenance marks from local text files.
//
// It covers Layer A only: invisible and format Unicode, space homoglyphs, and
// the document metadata that names a generator. Statistical (token-sampling)
// watermarks live in the word choices themselves and cannot be removed without
// rewriting the prose; pixel-domain marks need the image pipeline. Neither is
// in scope here, and the reports say so rather than implying a clean bill.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const usage = `wmr: inspect and strip AI provenance marks from local text files

Usage:
  wmr inspect [flags] [target ...]
  wmr clean   [flags] [target ...]
  wmr version
  wmr help

Targets are files, directories, or glob patterns. Add -r to search
directories and match patterns recursively. With no target, reads stdin.

  wmr inspect notes.md
  wmr inspect -r "*.go"
  wmr clean   -r ./docs -in-place

Run "wmr <command> -h" for the full flag list.

Exit codes:
  0  clean / no marks found
  1  marks found (inspect); cleaning still exits 0 on success
  2  usage error, I/O error, or no target matched any file
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	var code int
	switch os.Args[1] {
	case "inspect":
		code, err = runInspect(os.Args[2:])
	case "clean":
		code, err = runClean(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	case "version", "--version":
		fmt.Println("wmr " + version)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(code)
}

// version is overwritten at build time via
// -ldflags "-X main.version=$(git describe --tags --always)".
var version = "dev"

func displayName(path string) string {
	if path == "-" || path == "" {
		return "<stdin>"
	}
	return path
}

func ext(path string) string {
	if path == "-" {
		return ""
	}
	return filepath.Ext(path)
}
