package services

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/quill/backend/internal/models"
)

var (
	ErrSkillRegistryInvalid = errors.New("invalid skill registry")
	ErrUnknownSkill         = errors.New("unknown skill")
)

var expectedSkillNames = []string{
	"beta-reader",
	"copy-editor",
	"developmental-editor",
	"dialogue-and-voice",
	"genre-conventions",
	"line-editor",
	"literary-agent",
	"pacing-and-tension",
	"period-register",
	"pov-and-tense",
	"proofreader",
	"prose-economy",
	"sensitivity-reader",
	"show-dont-tell",
	"worldbuilding-and-exposition",
}

var expectedGenreReferenceNames = []string{
	"adventure", "coming-of-age", "cozy-mystery", "crime", "dystopian",
	"epic-fantasy", "fantasy", "gothic", "historical", "horror", "literary",
	"mystery", "paranormal", "romance", "romantasy", "science-fiction",
	"space-opera", "thriller", "urban-fantasy", "young-adult",
}

var defaultSkillNames = []string{
	"developmental-editor",
	"line-editor",
	"copy-editor",
	"proofreader",
	"beta-reader",
	"sensitivity-reader",
	"literary-agent",
	"genre-conventions",
}

type skillFrontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	GenreTags   []string `yaml:"genre_tags"`
	Stage       string   `yaml:"stage"`
}

type registeredSkill struct {
	frontmatter skillFrontmatter
	body        string
	refs        map[string]string
}

// SkillRegistry loads the immutable editorial assets once at startup. The
// registry deliberately exposes frontmatter and selected prompt context, not
// mutable file handles, so a request cannot alter the process-wide catalogue.
type SkillRegistry struct {
	dir    string
	skills map[string]registeredSkill
}

func NewSkillRegistry(dir string) (*SkillRegistry, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "./skills"
	}
	r := &SkillRegistry{dir: dir, skills: make(map[string]registeredSkill)}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *SkillRegistry) load() error {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return fmt.Errorf("%w: read skill directory %q: %v", ErrSkillRegistryInvalid, r.dir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(r.dir, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("%w: read %s: %v", ErrSkillRegistryInvalid, path, err)
		}
		frontmatter, body, err := parseSkillDocument(string(data))
		if err != nil {
			return fmt.Errorf("%w: %s: %v", ErrSkillRegistryInvalid, path, err)
		}
		if _, exists := r.skills[frontmatter.Name]; exists {
			return fmt.Errorf("%w: duplicate skill name %q", ErrSkillRegistryInvalid, frontmatter.Name)
		}
		for _, tag := range frontmatter.GenreTags {
			if _, ok := allowedGenreTags[tag]; !ok {
				return fmt.Errorf("%w: skill %q has unknown genre tag %q", ErrSkillRegistryInvalid, frontmatter.Name, tag)
			}
		}
		r.skills[frontmatter.Name] = registeredSkill{frontmatter: frontmatter, body: body, refs: make(map[string]string)}
	}

	genreSkill, ok := r.skills["genre-conventions"]
	if !ok {
		return fmt.Errorf("%w: genre-conventions skill is missing", ErrSkillRegistryInvalid)
	}
	refDir := filepath.Join(r.dir, "genre-conventions", "references")
	refs, err := os.ReadDir(refDir)
	if err != nil {
		return fmt.Errorf("%w: read genre references: %v", ErrSkillRegistryInvalid, err)
	}
	for _, entry := range refs {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".md")
		if _, ok := allowedGenreTags[name]; !ok {
			return fmt.Errorf("%w: unknown genre reference %q", ErrSkillRegistryInvalid, name)
		}
		content, err := os.ReadFile(filepath.Join(refDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("%w: read genre reference %q: %v", ErrSkillRegistryInvalid, name, err)
		}
		genreSkill.refs[name] = string(content)
	}
	r.skills["genre-conventions"] = genreSkill

	if err := validateExactNames(r.skills, expectedSkillNames, "skills"); err != nil {
		return err
	}
	if err := validateExactNames(genreSkill.refs, expectedGenreReferenceNames, "genre references"); err != nil {
		return err
	}
	return nil
}

func parseSkillDocument(document string) (skillFrontmatter, string, error) {
	if !strings.HasPrefix(document, "---\n") {
		return skillFrontmatter{}, "", errors.New("frontmatter must start with ---")
	}
	rest := document[len("---\n"):]
	boundary := strings.Index(rest, "\n---")
	if boundary < 0 {
		return skillFrontmatter{}, "", errors.New("frontmatter closing --- is missing")
	}
	frontmatterText := rest[:boundary]
	body := rest[boundary+len("\n---"):]
	body = strings.TrimPrefix(body, "\n")
	var frontmatter skillFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatterText), &frontmatter); err != nil {
		return skillFrontmatter{}, "", fmt.Errorf("parse frontmatter: %w", err)
	}
	frontmatter.Name = strings.TrimSpace(frontmatter.Name)
	frontmatter.Description = strings.TrimSpace(frontmatter.Description)
	frontmatter.Stage = strings.TrimSpace(frontmatter.Stage)
	if frontmatter.Name == "" || frontmatter.Description == "" || frontmatter.Stage == "" {
		return skillFrontmatter{}, "", errors.New("name, description, and stage are required")
	}
	if strings.TrimSpace(body) == "" {
		return skillFrontmatter{}, "", errors.New("skill body is required")
	}
	return frontmatter, body, nil
}

func validateExactNames[T any](items map[string]T, expected []string, label string) error {
	if len(items) != len(expected) {
		return fmt.Errorf("%w: expected %d %s, found %d", ErrSkillRegistryInvalid, len(expected), label, len(items))
	}
	for _, name := range expected {
		if _, ok := items[name]; !ok {
			return fmt.Errorf("%w: missing %s %q", ErrSkillRegistryInvalid, label, name)
		}
	}
	return nil
}

func (r *SkillRegistry) Get(name string) (body string, frontmatter models.SkillCatalogueItem, ok bool) {
	if r == nil {
		return "", models.SkillCatalogueItem{}, false
	}
	skill, ok := r.skills[name]
	if !ok {
		return "", models.SkillCatalogueItem{}, false
	}
	return skill.body, models.SkillCatalogueItem{
		Name: skill.frontmatter.Name, Description: skill.frontmatter.Description,
		GenreTags: append([]string{}, skill.frontmatter.GenreTags...), Stage: skill.frontmatter.Stage,
	}, true
}

func (r *SkillRegistry) Catalogue() []models.SkillCatalogueItem {
	if r == nil {
		return []models.SkillCatalogueItem{}
	}
	names := r.SkillNames()
	items := make([]models.SkillCatalogueItem, 0, len(names))
	for _, name := range names {
		_, item, _ := r.Get(name)
		items = append(items, item)
	}
	return items
}

func (r *SkillRegistry) SkillNames() []string {
	if r == nil {
		return []string{}
	}
	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *SkillRegistry) DefaultSkillNames() []string {
	return append([]string(nil), defaultSkillNames...)
}

func (r *SkillRegistry) ValidateNames(names []string) ([]string, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: registry is not configured", ErrSkillRegistryInvalid)
	}
	seen := make(map[string]struct{}, len(names))
	validated := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("%w: empty skill name", ErrUnknownSkill)
		}
		if _, ok := r.skills[name]; !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownSkill, name)
		}
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}
		validated = append(validated, name)
	}
	sort.Strings(validated)
	return validated, nil
}

func (r *SkillRegistry) PromptContext(names, genreTags []string) (string, error) {
	validated, err := r.ValidateNames(names)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, name := range validated {
		skill := r.skills[name]
		fmt.Fprintf(&b, "## Skill: %s\n%s\n\n%s\n\n", name, skill.frontmatter.Description, skill.body)
		if name != "genre-conventions" {
			continue
		}
		for _, tag := range genreTags {
			if ref, ok := skill.refs[tag]; ok {
				fmt.Fprintf(&b, "## Genre reference: %s\n%s\n\n", tag, ref)
			}
		}
	}
	return strings.TrimSpace(b.String()), nil
}
