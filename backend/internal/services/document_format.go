package services

import (
	"html"
	"regexp"
	"strings"
)

var (
	markdownStrongPattern = regexp.MustCompile(`(\*\*|__)([^*_\n]+)(\*\*|__)`)
	markdownEmPattern     = regexp.MustCompile(`(\*|_)([^*_\n]+)(\*|_)`)
	editorTagPattern      = regexp.MustCompile(`<[^>]+>`)
)

// MarkdownToEditorHTML converts the small, writer-facing Markdown subset we
// need at ingestion time into TipTap-native HTML. It deliberately has no
// network/parser dependency: paragraphs, italics, bold, headings, and scene
// breaks cover the fidelity promised by Sprint 6.
func MarkdownToEditorHTML(markdown string) string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	var out strings.Builder
	paragraph := make([]string, 0, 2)
	flush := func() {
		if len(paragraph) == 0 {
			return
		}
		parts := make([]string, len(paragraph))
		for i, line := range paragraph {
			parts[i] = markdownInlineHTML(line)
		}
		out.WriteString("<p>")
		out.WriteString(strings.Join(parts, "<br>"))
		out.WriteString("</p>")
		paragraph = paragraph[:0]
	}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			flush()
			continue
		}
		if isMarkdownSceneBreak(line) {
			flush()
			out.WriteString(`<hr data-scene-break="true">`)
			continue
		}
		if level, title, ok := markdownHeading(line); ok {
			flush()
			out.WriteString("<h")
			out.WriteString(string(rune('0' + level)))
			out.WriteString(">")
			out.WriteString(markdownInlineHTML(title))
			out.WriteString("</h")
			out.WriteString(string(rune('0' + level)))
			out.WriteString(">")
			continue
		}
		paragraph = append(paragraph, line)
	}
	flush()
	return out.String()
}

func stripEditorMarkup(value string) string {
	return strings.TrimSpace(html.UnescapeString(editorTagPattern.ReplaceAllString(value, " ")))
}

func markdownInlineHTML(value string) string {
	value = html.EscapeString(value)
	value = markdownStrongPattern.ReplaceAllString(value, "<strong>$2</strong>")
	return markdownEmPattern.ReplaceAllString(value, "<em>$2</em>")
}

func markdownHeading(line string) (int, string, bool) {
	for level := 1; level <= 3; level++ {
		prefix := strings.Repeat("#", level) + " "
		if strings.HasPrefix(line, prefix) {
			title := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			return level, title, title != ""
		}
	}
	return 0, "", false
}

func isMarkdownSceneBreak(line string) bool {
	switch strings.TrimSpace(line) {
	case "***", "* * *", "---", "___":
		return true
	default:
		return false
	}
}
