package services

import (
	"bytes"
	"fmt"
	"html"
	"strings"

	nethtml "golang.org/x/net/html"

	"github.com/quill/backend/internal/models"
)

// EditorHTMLToMarkdown converts TipTap HTML into human-diffable Markdown. It
// supports the nodes Quill currently stores and falls back to the original
// value for legacy plain-text chapters.
func EditorHTMLToMarkdown(content string) string {
	if !strings.Contains(content, "<") {
		return strings.TrimSpace(content)
	}
	root, err := nethtml.Parse(strings.NewReader(content))
	if err != nil {
		return strings.TrimSpace(content)
	}
	var out strings.Builder
	writeMarkdownNode(&out, root)
	return strings.TrimSpace(strings.ReplaceAll(out.String(), "\r\n", "\n"))
}

func writeMarkdownNode(out *strings.Builder, node *nethtml.Node) {
	if node == nil {
		return
	}
	switch node.Type {
	case nethtml.TextNode:
		out.WriteString(html.UnescapeString(node.Data))
		return
	case nethtml.ElementNode:
		switch node.Data {
		case "p":
			writeChildren(out, node)
			writeBlockBreak(out)
			return
		case "h1", "h2", "h3":
			level := int(node.Data[1] - '0')
			out.WriteString(strings.Repeat("#", level))
			out.WriteByte(' ')
			writeChildren(out, node)
			writeBlockBreak(out)
			return
		case "em", "i":
			out.WriteByte('*')
			writeChildren(out, node)
			out.WriteByte('*')
			return
		case "strong", "b":
			out.WriteString("**")
			writeChildren(out, node)
			out.WriteString("**")
			return
		case "br":
			out.WriteByte('\n')
			return
		case "hr":
			writeBlockBreak(out)
			out.WriteString("---")
			writeBlockBreak(out)
			return
		case "blockquote":
			writeChildren(out, node)
			writeBlockBreak(out)
			return
		case "ul", "ol":
			writeChildren(out, node)
			writeBlockBreak(out)
			return
		case "li":
			out.WriteString("- ")
			writeChildren(out, node)
			writeBlockBreak(out)
			return
		case "script", "style":
			return
		}
	}
	writeChildren(out, node)
}

func writeChildren(out *strings.Builder, node *nethtml.Node) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		writeMarkdownNode(out, child)
	}
}

func writeBlockBreak(out *strings.Builder) {
	value := out.String()
	if value == "" || strings.HasSuffix(value, "\n\n") {
		return
	}
	if strings.HasSuffix(value, "\n") {
		out.WriteByte('\n')
		return
	}
	out.WriteString("\n\n")
}

// ExportChapterMarkdown is a small service seam for handlers and tests.
func ExportChapterMarkdown(chapterTitle, content string) string {
	body := EditorHTMLToMarkdown(content)
	if strings.TrimSpace(chapterTitle) == "" {
		return body + "\n"
	}
	if body == "" {
		return fmt.Sprintf("# %s\n", strings.TrimSpace(chapterTitle))
	}
	return fmt.Sprintf("# %s\n\n%s\n", strings.TrimSpace(chapterTitle), body)
}

// ExportWorkMarkdown concatenates chapters in their repository order.
func ExportWorkMarkdown(title string, chapters []models.Chapter) string {
	var out bytes.Buffer
	if strings.TrimSpace(title) != "" {
		out.WriteString("# ")
		out.WriteString(strings.TrimSpace(title))
		out.WriteString("\n\n")
	}
	for i, chapter := range chapters {
		if i > 0 || strings.TrimSpace(title) != "" {
			// Chapter headings are level two in a work export so the work title
			// remains the document-level heading.
			out.WriteString("## ")
			out.WriteString(strings.TrimSpace(chapter.Title))
			out.WriteString("\n\n")
		} else {
			out.WriteString("# ")
			out.WriteString(strings.TrimSpace(chapter.Title))
			out.WriteString("\n\n")
		}
		out.WriteString(EditorHTMLToMarkdown(chapter.Content))
		out.WriteString("\n\n")
	}
	return strings.TrimSpace(out.String()) + "\n"
}
