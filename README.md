# watermarks-remover-go

[![build-and-publish](https://github.com/chriswirz/watermarks-remover-go/actions/workflows/release.yml/badge.svg)](https://github.com/chriswirz/watermarks-remover-go/actions/workflows/release.yml)

A single Go binary that finds and removes AI provenance marks from local
text files. It is a Go rewrite of the text half of
[watermarks-remover](https://github.com/guillaumemeyer/watermarks-remover):
same detection rules, idiomatic Go structure, no Python and no runtime
dependencies.

For privacy and hygiene on content **you own**.

## What it covers

| Layer | Target | Status |
| --- | --- | --- |
| **A** | Invisible Unicode, exotic spaces, bidi controls, tag chars, private use | Full |
| **Docs** | Markdown YAML frontmatter, HTML `<meta>` / JSON-LD / `data-ai*` | Full |
| **Lines** | AI attribution footers, disclaimers, generator comments | Opt-in, regex-based |
| **B** | Statistical (token-sampling) watermarks | Out of scope; needs a rewrite of the prose |
| **Images** | C2PA / EXIF / XMP, pixel-domain marks | Out of scope; see the Python original |

The reports say so explicitly rather than implying a clean bill of health: a
`Suspicious: 0` result means no *edit-based* carrier was found, not that the
text carries no watermark.

## Install

```sh
go install github.com/chriswirz/watermarks-remover-go/cmd/wmr@latest
```

Or build from a checkout:

```sh
go build -o wmr ./cmd/wmr     # or: make build
```

On Windows:

```bat
build.cmd            :: gofmt + vet + test, then build .\wmr.exe
build.cmd all        :: cross-compile all six targets into dist\ with SHA256SUMS
```

`build.cmd` is a thin wrapper around `build.ps1`; run the `.ps1` directly if
you prefer. Both stamp the version from `git describe`, or from `$env:VERSION`
if you set it.

### Linux packages

Each build publishes `.deb` and `.rpm` packages for amd64 and arm64 alongside
the raw binaries:

```sh
sudo dpkg -i wmr_0.0.42_amd64.deb        # Debian, Ubuntu
sudo rpm -i  wmr-0.0.42-1.x86_64.rpm     # Fedora, RHEL, openSUSE
```

They install `/usr/bin/wmr`, with this README and the license under
`/usr/share/doc/wmr/` (the license twice: as `copyright`, which Debian policy
expects, and as `LICENSE`). To
build one locally, stage the binary where nfpm expects it and run it against
[`packaging/nfpm.yaml`](packaging/nfpm.yaml):

```sh
go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o packaging/wmr ./cmd/wmr
ARCH=amd64 PKG_VERSION=0.0.1 nfpm package -f packaging/nfpm.yaml -p deb -t .
```

## Use

```sh
wmr inspect draft.md              # report what is in there; exit 1 if anything
wmr clean   draft.md              # writes draft.cleaned.md
wmr clean   draft.md -in-place    # overwrites, keeping draft.md.bak
wmr clean   *.md -in-place        # several files at once

wmr clean   ./docs -in-place      # files directly in a directory
wmr clean   -r ./docs -in-place   # ... and everything beneath it
wmr inspect -r "*.go"             # every .go file in this tree
wmr inspect "**/*.md"             # ** is recursive whatever -r says
wmr clean   -r . -dry-run         # report what would change, write nothing

cat draft.md | wmr clean > out.md # stdin/stdout
wmr inspect draft.md -json | jq . # machine-readable

wmr version                       # the build's version stamp
wmr help                          # command list; "wmr <cmd> -h" for flags
```

### Targets

Each positional argument is a file, a directory, or a glob pattern.
`-r` (or `-recursive`) controls the depth of **both** directory searches and
glob matching; a pattern containing `**` is recursive regardless.

| Target | Without `-r` | With `-r` |
| --- | --- | --- |
| `notes.md` | that file | that file |
| `./docs` | files directly in `./docs` | the whole tree |
| `"*.go"` | `.go` files here | every `.go` file beneath here |
| `"**/*.go"` | every `.go` file beneath here | same |

Directory searches collect known text extensions only; a glob pattern matches
whatever you asked for, since naming `*.xyz` is an instruction. Both skip
`.git`, `node_modules`, `vendor`, `dist`, `__pycache__` and friends. A pattern
that matches nothing warns; if *nothing* matched at all, that is exit 2 rather
than a silent success.

When more than one file is involved, a `Processed N files, modified M.` summary
closes the run, and unchanged files stay quiet unless you pass `-verbose`.

### Where output goes

| Situation | Destination |
| --- | --- |
| one file, no flags | `draft.md` → `draft.cleaned.md` |
| `-o PATH` | that path (one input only) |
| `-in-place` | the original, with a `.bak` beside it |
| stdin | stdout |
| more than one file | **must** pass `-in-place`, `-o`, or `-dry-run` |

That last row is a guard rather than a limitation. The `*.cleaned.*` default is
convenient for one named file and a menace across a tree: it scatters copies
through your source, and in a compiled language each copy is a duplicate of its
neighbour that breaks the build. So a multi-file run has to say where output
goes:

```
$ wmr clean -r .
refusing to write 31 separate .cleaned.* files into your tree; pass -in-place
to rewrite the originals (a .bak is kept), or -dry-run to see what would change
```

**Quote patterns on POSIX shells.** bash expands `*.go` itself before `wmr`
ever sees it, and its expansion is never recursive, so unquoted
`wmr inspect -r *.go` silently searches only the current directory. Quoting
hands the pattern to `wmr`, which does the recursive match:

```sh
wmr inspect -r '*.go'     # correct on bash/zsh
wmr inspect -r *.go       # only the current directory
```

cmd and PowerShell do not expand globs at all, so `.\wmr.exe inspect -r *.go`
works unquoted there.

Flags may appear before or after the filenames.

### Exit codes

| Code | Meaning |
| --- | --- |
| 0 | `inspect` found nothing; or `clean` succeeded |
| 1 | `inspect` found marks |
| 2 | usage error, I/O error, or no target matched any file |

So `wmr inspect` drops into a pre-commit hook or CI step directly. Note that a
run matching **no files** is exit 2, not a quiet success. A command that
processed nothing did not do what was asked, and a script should notice.

### Flags

Shared by `inspect` and `clean`:

| Flag | Effect |
| --- | --- |
| `-r`, `-recursive` | search directories and match globs recursively |
| `-json` | machine-readable output |
| `-verbose` | name every file, including the unaffected ones |
| `-aggressive` *(inspect)* / `-aggressive-homoglyphs` *(clean)* | Latin confusables and fullwidth lookalikes |
| `-strip-emoji-glue` | also treat load-bearing invisibles as suspect |
| `-no-metadata` | skip frontmatter / HTML metadata |
| `-attribution` | use the built-in AI attribution line patterns |
| `-patterns "re1,re2"` | use these line patterns instead of the built-ins |
| `-pattern-file FILE` | one regexp per line, added to whichever set is in use |
| `-strip-comments` | treat every single-line comment as removable |
| `-force-text` | proceed even when the input looks binary |

`inspect` only reports; `clean` also takes:

| Flag | Effect |
| --- | --- |
| `-o PATH` | output path (single input only) |
| `-in-place` | overwrite the input, keeping a `.bak` (also spelled `-inplace`) |
| `-dry-run` | report what would change, write nothing |
| `-nfkc` | apply Unicode NFKC after the scrub |
| `-keep-spaces` | leave exotic spaces alone instead of rewriting to U+0020 |
| `-strip-bidi` | also strip RTL/LTR directional marks and isolates |
| `-visible utf8\|ascii` | force visible characters only (see below) |
| `-stats` | full per-file breakdown on stderr |

Flags may appear before or after the targets, and take one dash or two. Where
a flag exists on both commands it means the same thing: on `inspect` it widens
what gets reported, on `clean` it widens what gets removed.

## Forcing visible-only text

The Layer A scrub is surgical: it removes the carriers it can name and
preserves anything load-bearing. `-visible` is the blunt instrument for when
you would rather not reason about which invisible is legitimate.

```sh
wmr clean draft.md -visible utf8    # printable Unicode, any script
wmr clean draft.md -visible ascii   # fold all the way down to ASCII
```

Nothing survives except visibly rendered glyphs and four whitespace
characters: **space, tab, CR and LF**. Tab is kept because it is structural in
code, Makefiles and TSV. Every other space-like character folds to a single
`U+0020`; line and paragraph separators become `
`.

| Input | `-visible utf8` | `-visible ascii` |
| --- | --- | --- |
| `“Café” — naïve… 😀 中` | `“Café” — naïve… 中` | `"Cafe" - naive...  ` |
| `中文 العربية` | `中文 العربية` | *(dropped)* |
| `a	b` | `a	b` | `a	b` |
| `★ ✓ ┌─┐` | `★ ✓ ┌─┐` | *(dropped)* |

**`utf8`** keeps printable Unicode from any script and drops control
characters, format characters, private-use and unassigned codepoints, the
supplementary emoji planes, and bytes that are not valid UTF-8. BMP symbols
such as `★ ✓ ☎ °` and the box-drawing characters are kept. They are visible
text that people use in prose and tables, and dropping them under a
"keep the visible characters" flag would be a nasty surprise.

**`ascii`** additionally folds to printable ASCII. Characters with a sensible
equivalent are transliterated (`é`→`e`, `“ ”`→`"`, `—`→`-`, `…`→`...`,
`½`→`1/2`, `©`→`(c)`, `æ`→`ae`, `ß`→`ss`), and anything with no ASCII form is
dropped. Accented words stay readable instead of losing letters.

Two things this mode deliberately does **not** do:

- It does not collapse runs of whitespace. Dropping a character leaves the
  spaces that surrounded it, which is why the table above shows two trailing
  spaces. Nothing else in this tool reflows text, and this is the wrong place
  to start.
- It does not respect the preservation rules. Emoji joiners, directional marks
  and script joiners are all invisible, so under `-visible` they all go, which
  means it **will** break emoji sequences and Persian or Devanagari
  orthography. That is the point of the flag; use the default clean if you
  need those intact.

## Removing attribution lines

The rune scrub and the metadata pass are surgical. This third layer deletes
**whole lines** that match a regular expression: the "Generated by ChatGPT"
footers, the "As an AI language model" disclaimers, the generator comments:

```sh
wmr inspect ./docs -attribution            # what would go
wmr clean   ./docs -attribution -dry-run   # same, in clean's own terms
wmr clean   ./docs -attribution -in-place  # actually do it
```

Bring your own patterns instead of, or as well as, the built-ins:

```sh
wmr clean draft.md -patterns '(?i)^draft,(?i)internal only'
wmr clean draft.md -attribution -pattern-file house-style.txt
```

`-patterns` **replaces** the built-in set; `-pattern-file` **adds** to whichever
set is in use. A pattern file is one regexp per line, with blank lines and
`#` comments skipped. Line endings are preserved exactly, so a CRLF file stays
CRLF and a file without a trailing newline does not gain one.

### The built-in patterns are blunt

They match **substrings, anywhere in a line**, with no notion of context.
`(?i)llm`, `(?i)claude`, `(?i)gemini` and `(?i)please verify` will remove a
sentence discussing a model, a variable named `claudeClient`, a citation, and
an ordinary instruction to check your work, exactly as readily as a generated
footer.

To put a number on it, point it at *this repository* and count:

```sh
wmr clean . -r -attribution -dry-run
```

At the time of writing that reports **89 lines** it would delete, including the
project's own test fixtures and the sentence in this README about not
misrepresenting AI-generated content. Run it yourself rather than trusting the
figure. The point is the order of magnitude, on a repository whose entire
subject matter is these vendor names.

That is the intended behavior when scrubbing attribution from your own
finished output. It is a poor fit for a source tree. So:

- Line removal is **off unless a flag turns it on**. Nothing deletes a line by
  default.
- `-dry-run` shows every match with the pattern that caught it, and writes
  nothing.
- `-in-place` still writes a `.bak` first.

`-strip-comments` is blunter still: it removes every single-line comment
regardless of content, which takes build tags, shebangs, linter pragmas and
license headers with it. Reach for it last.

## What it will not strip by default

Invisible does not mean unwanted. Several codepoints in the strip tables are
load-bearing in the right context, and removing them visibly damages the text:

- **Emoji glue.** A zero-width joiner between two emoji builds one glyph
  (👨‍👩‍👧, ❤️‍🔥); a variation selector picks the emoji or text presentation
  (⚖️). Stripped, the family splits into three people.
- **Script joiners.** ZWNJ inside Persian (می‌روم) or Devanagari is
  orthography, not a carrier.
- **Flag sequences.** A subdivision flag 🏴󠁧󠁢󠁳󠁣󠁴󠁿 is a base plus tag characters
  plus a terminator. A *complete* sequence is kept; loose tag chars are not.
- **Directional marks.** RLM/LRM and the isolates are legitimate in mixed
  RTL/LTR prose. Embeddings are kept only when properly paired; overrides are
  always stripped, since they can reorder unrelated spans.
- **Same-script fillers.** Mongolian free variation selectors, Khmer inherent
  vowels, Hangul jamo fillers, each meaningful only after a base from its own
  script.

Each of these is preserved only in the context that makes it meaningful; the
same codepoint free-floating is contraband and gets stripped. `inspect` still
reports what it preserved. `-strip-emoji-glue` and `-strip-bidi` remove them
anyway. Use those after reading the report, not before.

## Refusing binary input

Pointed at a `.docx` or a PNG, a text cleaner would decode the compressed
bytes, report whatever codepoints fell out (noise that tracks the
compression rather than the content), and then write the mangled bytes back,
destroying the file. `wmr` detects containers by magic number plus control-byte
density and refuses:

```
$ wmr inspect report.docx
refusing to treat report.docx as text: it looks like a ZIP container (DOCX, ODT, XLSX, PPTX, EPUB, JAR)
Pass --force-text to scan the raw bytes anyway (cleaning will corrupt the file).
```

Detection is conservative, so text in encodings other than UTF-8 keeps working.

## Safety

- A run touching more than one file must say where output goes: `-in-place`,
  `-o`, or `-dry-run`. The `*.cleaned.*` default is convenient for a single
  named file and a menace across a tree: it scatters copies through your
  source, and in a compiled language each copy is a duplicate of its neighbour
  that breaks the build.
- Searches skip the tool's own output (`*.cleaned.*`, `*.bak`), so cleaning a
  tree twice does not start cleaning the cleanings. An explicitly named path is
  still honored.
- Writes are atomic: a temp file in the destination directory, then a rename.
  An interrupted run never leaves a half-cleaned file in place.
- `-in-place` backs up the original *before* writing.
- Writing through a symlink is refused, so a pre-placed link cannot redirect
  the output onto another file.
- Input is size-capped, since everything is processed in memory.

## Layout

```
cmd/wmr/            CLI: argument handling, output formatting
internal/marks/     Layer A: the rune tables, classifier, scanner, reports
internal/docmeta/   Markdown frontmatter and HTML metadata
internal/lines/     Pattern-based whole-line removal
internal/textio/    Reading, binary refusal, atomic writes, backups, tree walk
packaging/          nfpm config for the .deb and .rpm packages
```

`internal/marks` runs one traversal (`scan`) that both `Inspect` and `Clean`
consume, so a hit the report names is exactly the rune the cleaner acts on.
The `-visible` pass in `visible.go` runs after that traversal as a separate,
deliberately blunter filter.

## Tests

```sh
go test ./...      # or: make test
```

The Layer A and document-metadata output was verified byte-for-byte against
the Python original on its own test fixtures.

## CI

`.github/workflows/release.yml` runs on every push and pull request:

1. **test**: `gofmt` check, `go vet`, `go test ./...`
2. **build**: static, `CGO_ENABLED=0` binaries for linux/macOS/Windows on
   amd64 and arm64, with the version stamped in via
   `-ldflags "-X main.version=$(git describe --tags --always)"`
3. **smoke**: downloads the real artifacts and runs each one on its native
   runner: strip a known payload, diff against the expected output, and check
   that `inspect` exits 1 before the clean and 0 after
4. **package**: builds `.deb` and `.rpm` for amd64 and arm64 with
   [nfpm](https://github.com/goreleaser/nfpm), from the Linux binaries the
   build job already produced
5. **publish**: pushes to `main`/`master` only. Collects every artifact,
   writes `SHA256SUMS`, and cuts a rolling `build-<run>` release

Pull requests run steps 1 to 3 and stop before packaging or publishing.

## Responsible use

For content you own or are authorized to modify. Stripping provenance from
someone else's work, or to misrepresent AI-generated content where disclosure
is required, is not what this is for.

## License

MIT. See [LICENSE](LICENSE).

This is a Go port of
[watermarks-remover](https://github.com/guillaumemeyer/watermarks-remover),
which is also MIT licensed; its copyright notice is retained in the LICENSE
file as that license requires.
