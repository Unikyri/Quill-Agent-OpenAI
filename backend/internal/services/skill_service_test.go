package services

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/quill/backend/internal/models"
)

func TestSkillRegistryLoadsAllSkillsAndGenreReferences(t *testing.T) {
	registry, err := NewSkillRegistry(filepath.Join("..", "..", "skills"))
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	if got := len(registry.Catalogue()); got != 15 {
		t.Fatalf("catalogue size = %d, want 15", got)
	}
	body, item, ok := registry.Get("genre-conventions")
	if !ok || body == "" || item.Description == "" {
		t.Fatal("genre-conventions frontmatter/body missing")
	}
	context, err := registry.PromptContext([]string{"genre-conventions"}, []string{"fantasy", "romance"})
	if err != nil {
		t.Fatalf("prompt context: %v", err)
	}
	if !strings.Contains(context, "## Genre reference: fantasy") || !strings.Contains(context, "## Genre reference: romance") {
		t.Fatalf("matching genre references missing from prompt context")
	}
	if strings.Contains(context, "## Genre reference: horror") {
		t.Fatalf("non-matching genre reference was loaded")
	}
}

func TestParseSkillDocumentRejectsMalformedFrontmatter(t *testing.T) {
	for name, document := range map[string]string{
		"missing opener":         "name: broken\n---\nbody",
		"missing closer":         "---\nname: broken\ndescription: no close\n",
		"missing required field": "---\nname: broken\nstage: craft\n---\nbody",
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := parseSkillDocument(document); err == nil {
				t.Fatal("expected malformed frontmatter error")
			}
		})
	}
}

func TestCraftRewriteOverlapAndSelectionCap(t *testing.T) {
	quote := "The old house stood silent beneath the moonlight"
	if !craftRewriteOverlap(quote, quote) {
		t.Fatal("identical note should be rejected as rewrite")
	}
	if craftRewriteOverlap("The image feels ominous here", quote) {
		t.Fatal("diagnostic note should not be rejected as rewrite")
	}
	selected := limitSelected([]string{"unknown", "line-editor", "line-editor", "beta-reader", "copy-editor", "proofreader"}, []string{"line-editor", "beta-reader", "copy-editor", "proofreader"})
	if len(selected) != maxCraftReviewSkills {
		t.Fatalf("selected count = %d, want %d", len(selected), maxCraftReviewSkills)
	}
}

func TestCraftReviewNotesReceiveFeedbackSafeIDs(t *testing.T) {
	service := &CraftReviewService{}
	notes := service.filterNotes(nil, uuid.New(), uuid.New(), "The old house stood silent beneath the moonlight", []string{"line-editor"}, []models.CraftReviewNote{{
		Skill: "line-editor", Quote: "The old house stood silent beneath the moonlight", Note: "The image creates an ominous pause.", Severity: "suggestion",
	}})
	if len(notes) != 1 || notes[0].ID == uuid.Nil {
		t.Fatalf("expected one note with a UUID, got %#v", notes)
	}
}
