package services

import "testing"

// TestFuseRRFBasicScore verifies the k=60 RRF math: an item ranked first
// (rank 0) in a single pipeline scores exactly 1/(60+1).
func TestFuseRRFBasicScore(t *testing.T) {
	list := []rankedEntry{{id: "a", fact: "fact a", source: "vector"}}

	result := fuseRRF(list)

	if len(result) != 1 {
		t.Fatalf("expected 1 fused item, got %d", len(result))
	}
	want := 1.0 / 61.0
	if result[0].RRFScore != want {
		t.Errorf("RRFScore = %v, want %v", result[0].RRFScore, want)
	}
	if result[0].ID != "a" || result[0].Fact != "fact a" {
		t.Errorf("unexpected fused item: %+v", result[0])
	}
}

// TestFuseRRFMultiPipelineOutranksSingle proves an item appearing rank-1 in
// two pipelines strictly outranks a distinct item appearing rank-1 in only
// one pipeline, and that dedupe-by-ID merges the shared item into one entry
// recording both sources.
func TestFuseRRFMultiPipelineOutranksSingle(t *testing.T) {
	graphList := []rankedEntry{{id: "shared", fact: "shared fact", source: "graph"}}
	recencyList := []rankedEntry{{id: "shared", fact: "shared fact", source: "recency"}}
	vectorList := []rankedEntry{{id: "solo", fact: "solo fact", source: "vector"}}

	result := fuseRRF(graphList, recencyList, vectorList)

	if len(result) != 2 {
		t.Fatalf("expected 2 fused items (deduped from 3 entries), got %d", len(result))
	}
	if result[0].ID != "shared" {
		t.Fatalf("expected 'shared' to rank first, got %q", result[0].ID)
	}
	if result[0].RRFScore <= result[1].RRFScore {
		t.Errorf("shared score %v should exceed solo score %v", result[0].RRFScore, result[1].RRFScore)
	}
	if len(result[0].Sources) != 2 {
		t.Errorf("expected 'shared' to record 2 sources, got %v", result[0].Sources)
	}
}

// TestFuseRRFTieBreakByIDAscending proves items tied on RRF score (rank 0 in
// their own single-item list each) are ordered by ID ascending for
// deterministic output.
func TestFuseRRFTieBreakByIDAscending(t *testing.T) {
	listA := []rankedEntry{{id: "zeta", fact: "z", source: "vector"}}
	listB := []rankedEntry{{id: "alpha", fact: "a", source: "keyword"}}

	result := fuseRRF(listA, listB)

	if len(result) != 2 {
		t.Fatalf("expected 2 fused items, got %d", len(result))
	}
	if result[0].RRFScore != result[1].RRFScore {
		t.Fatalf("expected tied scores, got %v vs %v", result[0].RRFScore, result[1].RRFScore)
	}
	if result[0].ID != "alpha" || result[1].ID != "zeta" {
		t.Errorf("expected tie-break alpha before zeta, got order: %q, %q", result[0].ID, result[1].ID)
	}
}
