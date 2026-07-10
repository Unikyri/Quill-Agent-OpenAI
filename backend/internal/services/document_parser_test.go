package services

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// buildTestDOCX generates a minimal .docx (a zip containing word/document.xml)
// in-test for determinism, instead of committing a binary fixture — matches
// the design's stated approach for this format.
func buildTestDOCX(t *testing.T, paragraphs []struct{ style, text string }) []byte {
	t.Helper()
	var body strings.Builder
	for _, p := range paragraphs {
		body.WriteString(`<w:p>`)
		if p.style != "" {
			body.WriteString(fmt.Sprintf(`<w:pPr><w:pStyle w:val="%s"/></w:pPr>`, p.style))
		}
		body.WriteString(fmt.Sprintf(`<w:r><w:t>%s</w:t></w:r>`, p.text))
		body.WriteString(`</w:p>`)
	}
	documentXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>` + body.String() + `</w:body>
</w:document>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := f.Write([]byte(documentXML)); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// buildTestPDF generates a minimal single-page valid PDF containing the given
// text, computing correct xref byte offsets — hand-crafted structure per the
// design, generated in-code (rather than a committed binary) so the exact
// byte offsets are always correct and the fixture is trivially editable.
func buildTestPDF(text string) []byte {
	var buf bytes.Buffer
	offsets := make([]int, 6) // index 1..5 used

	write := func(s string) { buf.WriteString(s) }

	write("%PDF-1.4\n")

	offsets[1] = buf.Len()
	write("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	offsets[2] = buf.Len()
	write("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	offsets[3] = buf.Len()
	write("3 0 obj\n<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 4 0 R >> >> /MediaBox [0 0 612 792] /Contents 5 0 R >>\nendobj\n")

	offsets[4] = buf.Len()
	write("4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")

	stream := fmt.Sprintf("BT /F1 24 Tf 72 712 Td (%s) Tj ET", text)
	offsets[5] = buf.Len()
	write(fmt.Sprintf("5 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(stream), stream))

	xrefStart := buf.Len()
	write("xref\n0 6\n")
	write("0000000000 65535 f \n")
	for i := 1; i <= 5; i++ {
		write(fmt.Sprintf("%010d 00000 n \n", offsets[i]))
	}
	write("trailer\n<< /Size 6 /Root 1 0 R >>\n")
	write(fmt.Sprintf("startxref\n%d\n%%%%EOF", xrefStart))

	return buf.Bytes()
}

func TestFileTypeOf(t *testing.T) {
	cases := map[string]string{
		"manuscript.md":   "md",
		"manuscript.TXT":  "txt",
		"manuscript.docx": "docx",
		"manuscript.PDF":  "pdf",
		"manuscript.doc":  "doc",
		"noext":           "",
	}
	for name, want := range cases {
		if got := fileTypeOf(name); got != want {
			t.Errorf("fileTypeOf(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestParseDocumentPassthrough(t *testing.T) {
	for _, ext := range []string{"md", "txt"} {
		filename := "manuscript." + ext
		text, err := parseDocument(filename, []byte("Hello world."))
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", filename, err)
		}
		if text != "Hello world." {
			t.Errorf("%s: text = %q, want %q", filename, text, "Hello world.")
		}
	}
}

func TestParseDocumentStripsBOM(t *testing.T) {
	content := append([]byte(utf8BOM), []byte("Hello world.")...)
	text, err := parseDocument("manuscript.txt", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "Hello world." {
		t.Errorf("text = %q, want BOM stripped %q", text, "Hello world.")
	}
}

func TestParseDocumentRejectsLegacyDoc(t *testing.T) {
	_, err := parseDocument("manuscript.doc", []byte("binary junk"))
	if err == nil {
		t.Fatal("expected error for .doc, got nil")
	}
}

func TestParseDocumentRejectsUnknownExtension(t *testing.T) {
	_, err := parseDocument("manuscript.rtf", []byte("binary junk"))
	if err == nil {
		t.Fatal("expected error for unknown extension, got nil")
	}
}

func TestParseDOCX(t *testing.T) {
	content := buildTestDOCX(t, []struct{ style, text string }{
		{style: "Heading1", text: "Chapter One"},
		{text: "Bilbo was going to have a birthday party."},
		{style: "Heading2", text: "A Shorter Heading"},
		{text: "Frodo learns about the Ring."},
	})

	text, err := parseDocument("manuscript.docx", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "# Chapter One") {
		t.Errorf("expected Heading1 prefixed with '# ', got: %q", text)
	}
	if !strings.Contains(text, "## A Shorter Heading") {
		t.Errorf("expected Heading2 prefixed with '## ', got: %q", text)
	}
	if !strings.Contains(text, "Bilbo was going to have a birthday party.") {
		t.Errorf("missing plain paragraph text, got: %q", text)
	}
}

// TestParseDOCXDecompressionBomb verifies a .docx whose word/document.xml
// decompresses past the ceiling is rejected with a clear error rather than
// streamed unbounded into memory (OOM). The ceiling is lowered to a few KB for
// the test so no multi-GB fixture is needed — the highly compressible body is
// tiny on disk but its uncompressed size trips the cap.
func TestParseDOCXDecompressionBomb(t *testing.T) {
	orig := maxDecompressedDOCXBytes
	maxDecompressedDOCXBytes = 4 << 10 // 4KB test-only ceiling
	defer func() { maxDecompressedDOCXBytes = orig }()

	// A valid document.xml wrapper around a large, highly compressible run of
	// bytes — its uncompressed size far exceeds the lowered ceiling.
	documentXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body><w:p><w:r><w:t>` + strings.Repeat("A", 64<<10) + `</w:t></w:r></w:p></w:body>
</w:document>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := f.Write([]byte(documentXML)); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	// The compressed .docx is only a few KB — a real bomb would be too, which
	// is exactly why an on-disk size check is not enough.
	if buf.Len() > 8<<10 {
		t.Fatalf("test fixture unexpectedly large (%d bytes); repeated bytes should compress tiny", buf.Len())
	}

	_, err = parseDocument("bomb.docx", buf.Bytes())
	if err == nil {
		t.Fatal("expected an error for an over-cap .docx, got nil (would OOM in prod)")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("expected a 'too large when decompressed' error, got: %v", err)
	}
}

func TestParseDOCXNotAZip(t *testing.T) {
	_, err := parseDocument("manuscript.docx", []byte("not a zip file"))
	if err == nil {
		t.Fatal("expected error for non-zip content, got nil")
	}
}

func TestParseDOCXMissingDocumentXML(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create("word/other.xml")
	f.Write([]byte("<xml/>"))
	zw.Close()

	_, err := parseDocument("manuscript.docx", buf.Bytes())
	if err == nil {
		t.Fatal("expected error for missing word/document.xml, got nil")
	}
}

func TestParsePDFValid(t *testing.T) {
	pdfBytes := buildTestPDF("Hello World")
	text, err := parseDocument("manuscript.pdf", pdfBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Hello World") {
		t.Errorf("expected extracted text to contain %q, got: %q", "Hello World", text)
	}
}

// TestParsePDFCorrupt exercises the recover() path in parsePDF: the
// ledongthuc/pdf parser is known to panic on malformed input, and a truncated
// PDF (valid header, then garbage) is a reliable way to trigger that.
func TestParsePDFCorrupt(t *testing.T) {
	valid := buildTestPDF("Hello World")
	corrupt := valid[:len(valid)/2]

	_, err := parseDocument("manuscript.pdf", corrupt)
	if err == nil {
		t.Fatal("expected error for corrupt PDF, got nil")
	}
}

func TestParsePDFNotAPDF(t *testing.T) {
	_, err := parseDocument("manuscript.pdf", []byte("this is not a pdf at all"))
	if err == nil {
		t.Fatal("expected error for non-PDF content, got nil")
	}
}

func TestParseDocumentEmptyExtraction(t *testing.T) {
	text, err := parseDocument("manuscript.md", []byte("   \n\n  "))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(text) != "" {
		t.Errorf("expected whitespace-only passthrough, got: %q", text)
	}
	// Note: parseDocument itself does not reject empty/whitespace-only text —
	// that check lives in runWorker (D1), which is format-agnostic and also
	// catches parse results like an empty DOCX/PDF.
}
