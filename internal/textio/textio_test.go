package textio

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLooksBinaryDetectsContainers(t *testing.T) {
	cases := map[string][]byte{
		"a ZIP container (DOCX, ODT, XLSX, PPTX, EPUB, JAR)": []byte("PK\x03\x04rest"),
		"a PDF":         []byte("%PDF-1.7\nstuff"),
		"a PNG image":   []byte("\x89PNG\r\n\x1a\n..."),
		"a JPEG image":  []byte("\xff\xd8\xff\xe0"),
		"an ELF binary": []byte("\x7fELF\x02\x01"),
	}
	for want, data := range cases {
		if got := LooksBinary(data); got != want {
			t.Errorf("LooksBinary(%q) = %q, want %q", data[:4], got, want)
		}
	}
}

func TestLooksBinaryAcceptsText(t *testing.T) {
	for _, s := range []string{
		"",
		"plain ascii\n",
		"UTF-8 with invisibles: a​b\n",
		"tabs\tand\r\nline endings\n",
		"latin-1 bytes: \xe9\xe8\xea caf\xe9\n", // not UTF-8, still text
	} {
		if got := LooksBinary([]byte(s)); got != "" {
			t.Errorf("LooksBinary(%q) = %q, want text", s, got)
		}
	}
}

func TestLooksBinaryDetectsNULAndControlDensity(t *testing.T) {
	if got := LooksBinary([]byte("abc\x00def")); !strings.Contains(got, "NUL") {
		t.Errorf("NUL not detected: %q", got)
	}
	dense := strings.Repeat("\x01\x02\x03", 100)
	if got := LooksBinary([]byte(dense)); !strings.Contains(got, "control") {
		t.Errorf("control density not detected: %q", got)
	}
}

func TestReadTextRefusesBinary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.docx")
	if err := os.WriteFile(path, []byte("PK\x03\x04payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadText(path, false)
	var binErr *ErrBinary
	if !errors.As(err, &binErr) {
		t.Fatalf("err = %v, want *ErrBinary", err)
	}
	if !strings.Contains(binErr.Error(), "--force-text") {
		t.Errorf("error does not name the override: %v", binErr)
	}

	if _, err := ReadText(path, true); err != nil {
		t.Errorf("--force-text still refused: %v", err)
	}
}

func TestReadTextRejectsOversizeInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.txt")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := MaxInputBytes
	MaxInputBytes = 4
	defer func() { MaxInputBytes = old }()

	if _, err := ReadText(path, false); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("err = %v, want a size refusal", err)
	}
}

func TestWriteFileAtomicLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := WriteFileAtomic(path, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "out.txt" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("directory contains %v, want just out.txt", names)
	}
	if got, _ := os.ReadFile(path); string(got) != "hello" {
		t.Errorf("content = %q", got)
	}
}

func TestWriteFileAtomicOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(path, []byte("old content here"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Errorf("content = %q, want %q", got, "new")
	}
}

func TestBackupPreservesOriginal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	bak, err := Backup(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(bak); string(got) != "original" {
		t.Errorf("backup content = %q", got)
	}
	if got, _ := os.ReadFile(path); string(got) != "original" {
		t.Errorf("Backup modified the source: %q", got)
	}
}

func TestCleanedPath(t *testing.T) {
	for in, want := range map[string]string{
		"draft.md":       "draft.cleaned.md",
		"a/b/notes.txt":  "a/b/notes.cleaned.txt",
		"noext":          "noext.cleaned",
		"archive.tar.gz": "archive.tar.cleaned.gz",
	} {
		if got := CleanedPath(in); got != want {
			t.Errorf("CleanedPath(%q) = %q, want %q", in, got, want)
		}
	}
}
