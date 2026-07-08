package eval

import (
	"encoding/json"
	"os"

	"github.com/google/uuid"
)

// GoldSet represents a loaded gold corpus with queries and a forgetting timeline.
type GoldSet struct {
	Queries              []GoldQuery      `json:"queries"`
	Forgetting           ForgettingLabels `json:"forgetting_timeline"`
	ConsolidationTargets []string         `json:"consolidation_targets"`
}

// GoldQuery is a single test query with relevance judgments.
type GoldQuery struct {
	ID                  string      `json:"id"`
	Query               string      `json:"query"`
	RelevantEntityNames []string    `json:"relevant_entity_names"`
	Note                string      `json:"note"`
	RelevantEntityIDs   []uuid.UUID `json:"-"`
}

// ForgettingLabels defines which entities should survive or be forgotten.
type ForgettingLabels struct {
	TotalChapters       int         `json:"total_chapters"`
	ShouldBeArchived    []string    `json:"should_be_archived"`
	MustStayActive      []string    `json:"must_stay_active"`
	ShouldBeArchivedIDs []uuid.UUID `json:"-"`
	MustStayActiveIDs   []uuid.UUID `json:"-"`
}

// LoadGold reads and parses a gold corpus JSON file.
func LoadGold(path string) (*GoldSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var g GoldSet
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, err
	}
	return &g, nil
}
