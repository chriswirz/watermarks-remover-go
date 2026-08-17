// Package textio reads and writes the text files the cleaner operates on,
// refusing binary input and never clobbering the original on failure.
package textio

import "bytes"

// binaryMagic are containers that get mistaken for text on the command line.
// Decoding one as text walks compressed bytes and reports whatever codepoints
// fall out of them, noise that tracks the compression rather than the content, and
// cleaning such a "text" writes the mangled bytes back and destroys the file.
var binaryMagic = []struct {
	magic []byte
	label string
}{
	{[]byte("PK\x03\x04"), "a ZIP container (DOCX, ODT, XLSX, PPTX, EPUB, JAR)"},
	{[]byte("PK\x05\x06"), "an empty ZIP container"},
	{[]byte("PK\x07\x08"), "a spanned ZIP container"},
	{[]byte("%PDF-"), "a PDF"},
	{[]byte("\x89PNG\r\n\x1a\n"), "a PNG image"},
	{[]byte("\xff\xd8\xff"), "a JPEG image"},
	{[]byte("GIF87a"), "a GIF image"},
	{[]byte("GIF89a"), "a GIF image"},
	{[]byte("II*\x00"), "a TIFF image"},
	{[]byte("MM\x00*"), "a TIFF image"},
	{[]byte("RIFF"), "a RIFF container (WEBP, WAV, AVI)"},
	{[]byte("OggS"), "an Ogg media file"},
	{[]byte("\x1f\x8b"), "a gzip archive"},
	{[]byte("BZh"), "a bzip2 archive"},
	{[]byte("\xfd7zXZ\x00"), "an xz archive"},
	{[]byte("7z\xbc\xaf\x27\x1c"), "a 7-Zip archive"},
	{[]byte("Rar!\x1a\x07"), "a RAR archive"},
	{[]byte("\x7fELF"), "an ELF binary"},
	{[]byte("\xca\xfe\xba\xbe"), "a Java class or Mach-O fat binary"},
	{[]byte("\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1"), "a legacy Office document (.doc, .xls, .ppt)"},
	{[]byte("SQLite format 3\x00"), "a SQLite database"},
	{[]byte("8BPS"), "a Photoshop document"},
	{[]byte("wOFF"), "a WOFF font"},
	{[]byte("wOF2"), "a WOFF2 font"},
	{[]byte("OTTO"), "an OpenType font"},
	{[]byte("\x00\x01\x00\x00\x00"), "a TrueType font"},
}

const sniffBytes = 8192

// Real text runs near 0% control bytes; compressed and executable data runs
// far above this. Tab, LF, VT, FF, CR and ESC are legitimate in text.
const controlRatioLimit = 0.05

func allowedControl(b byte) bool {
	switch b {
	case 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x1B:
		return true
	}
	return false
}

// LooksBinary describes why data is not plausibly text, or returns "" when it
// looks like text. Deliberately conservative: encodings other than UTF-8 must
// keep working, so bytes that fail to decode are not on their own proof.
func LooksBinary(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	for _, m := range binaryMagic {
		if bytes.HasPrefix(data, m.magic) {
			return m.label
		}
	}
	head := data
	if len(head) > sniffBytes {
		head = head[:sniffBytes]
	}
	if bytes.IndexByte(head, 0) >= 0 {
		return "binary data (contains NUL bytes)"
	}
	controls := 0
	for _, b := range head {
		if b < 0x20 && !allowedControl(b) {
			controls++
		}
	}
	if float64(controls)/float64(len(head)) > controlRatioLimit {
		return "binary data (dense in control bytes)"
	}
	return ""
}
