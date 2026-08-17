package textio

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// MaxInputBytes caps whole-file reads. Everything is processed in memory, so
// an uncapped read on a crafted path is a host-memory DoS.
var MaxInputBytes int64 = 256 << 20

// ErrBinary is returned when input looks like a binary container and the
// caller has not opted into scanning it anyway.
type ErrBinary struct {
	Origin string
	Kind   string
}

func (e *ErrBinary) Error() string {
	return fmt.Sprintf("refusing to treat %s as text: it looks like %s\n"+
		"Pass --force-text to scan the raw bytes anyway (cleaning will corrupt the file).",
		e.Origin, e.Kind)
}

// ReadText reads path (or stdin when path is "" or "-") as UTF-8 text.
// Invalid UTF-8 is preserved byte-for-byte by Go's string type, so text in
// other encodings survives a round trip untouched outside the ASCII range.
func ReadText(path string, allowBinary bool) (string, error) {
	origin := path
	var data []byte
	var err error

	if path == "" || path == "-" {
		origin = "stdin"
		data, err = io.ReadAll(io.LimitReader(os.Stdin, MaxInputBytes+1))
	} else {
		if info, statErr := os.Stat(path); statErr == nil && info.Size() > MaxInputBytes {
			return "", fmt.Errorf("refusing input larger than %d bytes: %s", MaxInputBytes, path)
		}
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return "", err
	}
	if int64(len(data)) > MaxInputBytes {
		return "", fmt.Errorf("refusing input larger than %d bytes: %s", MaxInputBytes, origin)
	}
	if !allowBinary {
		if kind := LooksBinary(data); kind != "" {
			return "", &ErrBinary{Origin: origin, Kind: kind}
		}
	}
	return string(data), nil
}

// WriteText writes text to path, or to stdout when path is "" or "-".
func WriteText(path, text string) error {
	if path == "" || path == "-" {
		if _, err := io.WriteString(os.Stdout, text); err != nil {
			return err
		}
		if text != "" && !strings.HasSuffix(text, "\n") {
			_, err := io.WriteString(os.Stdout, "\n")
			return err
		}
		return nil
	}
	return WriteFileAtomic(path, []byte(text))
}

// WriteFileAtomic writes data to path via a temp file in the same directory
// and a rename, so an interrupted write never leaves a half-cleaned file in
// place. It refuses to write through a symlink: a pre-placed link in a temp
// or download directory would otherwise redirect the output onto a victim file.
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to write through symlink: %s", path)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// CreateTemp makes the file 0600; restore normal permissions.
	if err := os.Chmod(tmpName, 0o644); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(tmpName, path)
}

// Backup copies src to src+".bak" and returns the backup path. Used by
// in-place flows so the original survives until the cleaned output lands.
func Backup(src string) (string, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	bak := src + ".bak"
	if err := WriteFileAtomic(bak, data); err != nil {
		return "", fmt.Errorf("cannot create backup %s: %w", bak, err)
	}
	return bak, nil
}

// CleanedPath maps path/to/file.ext to path/to/file.cleaned.ext.
func CleanedPath(src string) string {
	ext := filepath.Ext(src)
	return strings.TrimSuffix(src, ext) + ".cleaned" + ext
}

// statPath is os.Stat, split out so Walk can be tested against a stub.
func statPath(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// IsDir reports whether path is a directory. A path that cannot be stat'd is
// reported as not a directory; the subsequent read produces the real error.
func IsDir(path string) bool {
	if path == "" || path == "-" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
