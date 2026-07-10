package services

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

// fileTypeOf returns the lowercased file extension without the leading dot,
// e.g. "manuscript.PDF" -> "pdf". Empty for filenames with no extension.
func fileTypeOf(filename string) string {
	ext := filepath.Ext(filename)
	return strings.ToLower(strings.TrimPrefix(ext, "."))
}

// parseDocument dispatches on file extension and returns plain text suitable
// for chapter splitting. Raw binary content must never reach the caller —
// unsupported formats and parse failures return an error instead.
func parseDocument(filename string, content []byte) (string, error) {
	switch fileTypeOf(filename) {
	case "md", "txt":
		return stripBOM(string(content)), nil
	case "docx":
		return parseDOCX(content)
	case "pdf":
		return parsePDF(content)
	default:
		return "", fmt.Errorf("unsupported file type %q (only .md, .txt, .docx, .pdf are supported)", fileTypeOf(filename))
	}
}

// utf8BOM is the 3-byte UTF-8 byte-order-mark, written as an escaped byte
// sequence (not a literal rune) so gofmt/the compiler don't mistake it for a
// file-leading BOM, which is only meaningful at byte offset 0.
const utf8BOM = "\xEF\xBB\xBF"

// stripBOM removes a leading UTF-8 byte-order-mark, if present.
func stripBOM(s string) string {
	return strings.TrimPrefix(s, utf8BOM)
}

// ponytail: 50MB of decompressed document.xml is far beyond any real
// manuscript. This ceiling guards against a decompression bomb (a tiny .docx
// that expands to gigabytes and OOMs the shared ingestion process). It caps
// both the declared uncompressed size and the bytes actually streamed, so a
// lying central directory can't bypass it. Package-level var so tests can
// lower it without writing a multi-GB fixture.
var maxDecompressedDOCXBytes int64 = 50 << 20

// maxDOCXZipEntries rejects an absurd central directory (zip-bomb defense in
// depth) before we even scan for word/document.xml. A real .docx has a
// handful of entries, not thousands.
const maxDOCXZipEntries = 1000

// parseDOCX extracts plain text from a .docx file's word/document.xml using
// only the stdlib (archive/zip + encoding/xml) — no external dependency.
// Paragraphs styled Heading1/Heading2/Heading3 are prefixed with markdown
// heading markers so the existing splitChunks markdown pattern picks them up.
func parseDOCX(content []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("not a valid .docx (legacy .doc? Save as .docx): %w", err)
	}

	if len(zr.File) > maxDOCXZipEntries {
		return "", fmt.Errorf("not a valid .docx: too many zip entries (%d)", len(zr.File))
	}

	var docFile *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return "", fmt.Errorf("not a valid .docx: missing word/document.xml")
	}
	if docFile.UncompressedSize64 > uint64(maxDecompressedDOCXBytes) {
		return "", fmt.Errorf("document too large when decompressed (%d bytes exceeds %d byte cap)", docFile.UncompressedSize64, maxDecompressedDOCXBytes)
	}

	rc, err := docFile.Open()
	if err != nil {
		return "", fmt.Errorf("open word/document.xml: %w", err)
	}
	defer rc.Close()

	var buf strings.Builder
	// Defense in depth: cap the bytes actually streamed in case the central
	// directory under-reported UncompressedSize64.
	dec := xml.NewDecoder(io.LimitReader(rc, maxDecompressedDOCXBytes+1))

	var curStyle string
	var curText strings.Builder
	inText := false

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("parse word/document.xml: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				curStyle = ""
				curText.Reset()
			case "pStyle":
				for _, a := range t.Attr {
					if a.Name.Local == "val" {
						curStyle = a.Value
					}
				}
			case "t":
				inText = true
			}
		case xml.CharData:
			if inText {
				curText.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				text := curText.String()
				switch curStyle {
				case "Heading1":
					buf.WriteString("# ")
				case "Heading2":
					buf.WriteString("## ")
				case "Heading3":
					buf.WriteString("## ")
				}
				buf.WriteString(text)
				buf.WriteString("\n\n")
				if int64(buf.Len()) > maxDecompressedDOCXBytes {
					return "", fmt.Errorf("document too large when decompressed (exceeds %d byte cap)", maxDecompressedDOCXBytes)
				}
			}
		}
	}

	return strings.TrimSpace(buf.String()), nil
}

// parsePDF extracts plain text from a PDF using github.com/ledongthuc/pdf.
//
// recover() is mandatory: the underlying parser is known to panic on
// malformed PDFs (see its many internal `panic(...)` calls in read.go/page.go).
// A recovered panic is treated as a parse error so the ingestion job fails
// cleanly instead of crashing the worker goroutine.
func parsePDF(content []byte) (text string, err error) {
	defer func() {
		if r := recover(); r != nil {
			text = ""
			err = fmt.Errorf("malformed PDF (parser panic: %v)", r)
		}
	}()

	reader, rerr := pdf.NewReader(bytes.NewReader(content), int64(len(content)))
	if rerr != nil {
		return "", fmt.Errorf("PDF is encrypted or unreadable: %w", rerr)
	}

	plain, rerr := reader.GetPlainText()
	if rerr != nil {
		return "", fmt.Errorf("extract PDF text: %w", rerr)
	}

	b, rerr := io.ReadAll(plain)
	if rerr != nil {
		return "", fmt.Errorf("read extracted PDF text: %w", rerr)
	}

	extracted := strings.TrimSpace(string(b))
	if extracted == "" {
		return "", fmt.Errorf("no extractable text — scanned or image-only PDF? OCR is not supported")
	}

	return extracted, nil
}
