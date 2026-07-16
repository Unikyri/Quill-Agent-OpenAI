package main

import (
	"strings"
	"testing"
)

func TestFixtureContentIsDeterministicAtSprintSizes(t *testing.T) {
	for _, pages := range []int{50, 400} {
		fixture := fixtureContent(pages)
		if got := strings.Count(fixture, "# Chapter "); got != pages {
			t.Fatalf("pages=%d chunks=%d", pages, got)
		}
		if strings.Count(fixture, "James Holden") != pages*5 {
			t.Fatalf("pages=%d did not preserve repeated James Holden fixture mentions", pages)
		}
		// Five "James Holden" pairs plus 370 prose words make 380 content
		// fields; the Markdown heading contributes "#", "Chapter", and its number.
		if got, want := len(strings.Fields(fixture)), pages*383; got != want {
			t.Fatalf("pages=%d words=%d, want %d", pages, got, want)
		}
		if fixture != fixtureContent(pages) {
			t.Fatalf("pages=%d fixture is not deterministic", pages)
		}
	}
}

func TestPercentileUsesConservativeNearestRank(t *testing.T) {
	values := []float64{1, 2, 3}
	if got := percentile(values, .50); got != 2 {
		t.Fatalf("p50=%v, want 2", got)
	}
	if got := percentile(values, .95); got != 3 {
		t.Fatalf("p95=%v, want 3", got)
	}
}
