package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeIngestionTicker struct {
	ch      chan time.Time
	stopped bool
}

func (t *fakeIngestionTicker) C() <-chan time.Time { return t.ch }
func (t *fakeIngestionTicker) Stop()               { t.stopped = true }

func TestIngestionProgressETAStartsAfterTenPercent(t *testing.T) {
	now := time.Unix(0, 0)
	svc := &IngestionService{progressNow: func() time.Time { return now }}
	tracker := newIngestionProgressTracker(svc, uuid.New(), uuid.New(), 20)
	now = now.Add(5 * time.Second)
	tracker.markMap(1, "Extracting")
	_, _, _, _, _, eta := tracker.snapshot()
	if eta != nil {
		t.Fatalf("ETA before 10%% = %v, want nil", *eta)
	}
	now = now.Add(15 * time.Second)
	tracker.markMap(4, "Extracting")
	_, _, _, _, _, eta = tracker.snapshot()
	if eta == nil || *eta <= 0 {
		t.Fatalf("ETA at 10%% = %v, want positive", eta)
	}
}

func TestIngestionProgressIsMonotonicAcrossOutOfOrderMapAndReduce(t *testing.T) {
	svc := &IngestionService{}
	tracker := newIngestionProgressTracker(svc, uuid.New(), uuid.New(), 4)
	tracker.markMap(3, "Extracting chapter three")
	_, processed, total, _, _, _ := tracker.snapshot()
	if processed != 3 || total != 8 {
		t.Fatalf("MAP snapshot = %d/%d, want 3/8", processed, total)
	}
	tracker.markMap(1, "Late completion")
	_, processed, _, _, _, _ = tracker.snapshot()
	if processed != 3 {
		t.Fatalf("out-of-order MAP regressed to %d", processed)
	}
	tracker.markMap(4, "MAP complete")
	tracker.markReduce(1, 2, "Saving chapter one")
	_, processed, total, _, _, _ = tracker.snapshot()
	if processed != 5 || total != 8 {
		t.Fatalf("REDUCE snapshot = %d/%d, want 5/8", processed, total)
	}
}

func TestIngestionProgressTickerPublishesAndStopsAtTerminal(t *testing.T) {
	ticker := &fakeIngestionTicker{ch: make(chan time.Time, 2)}
	hub := &mockIngestionHub{}
	svc := &IngestionService{hub: hub, newProgressTicker: func(time.Duration) ingestionTicker { return ticker }}
	tracker := newIngestionProgressTracker(svc, uuid.New(), uuid.New(), 4)
	tracker.start()
	tracker.publish()
	before := len(hub.popMessages())
	tracker.finish("completed", 4, 1, "Ingestion complete.")
	if !ticker.stopped {
		t.Fatal("ticker was not stopped at terminal status")
	}
	if before == 0 {
		t.Fatal("ticker tracker did not publish running progress")
	}
}
