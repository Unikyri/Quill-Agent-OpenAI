package services

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

const ingestionProgressInterval = 2 * time.Second

type ingestionTicker interface {
	C() <-chan time.Time
	Stop()
}
type realIngestionTicker struct{ *time.Ticker }

func (t realIngestionTicker) C() <-chan time.Time { return t.Ticker.C }

type ingestionProgressTracker struct {
	svc                                     *IngestionService
	jobID, userID                           uuid.UUID
	chaptersTotal                           int
	now                                     func() time.Time
	newTicker                               func(time.Duration) ingestionTicker
	mu                                      sync.Mutex
	status, action                          string
	mapProcessed, reduceProcessed, entities int
	lastAt                                  time.Time
	lastProcessed                           int
	ewmaSeconds                             float64
	done                                    chan struct{}
	stopOnce                                sync.Once
	wg                                      sync.WaitGroup
}

func newIngestionProgressTracker(svc *IngestionService, jobID, userID uuid.UUID, total int) *ingestionProgressTracker {
	now := svc.progressNow
	if now == nil {
		now = time.Now
	}
	newTicker := svc.newProgressTicker
	if newTicker == nil {
		newTicker = func(d time.Duration) ingestionTicker { return realIngestionTicker{time.NewTicker(d)} }
	}
	return &ingestionProgressTracker{svc: svc, jobID: jobID, userID: userID, chaptersTotal: total, now: now, newTicker: newTicker, status: "running", action: "Preparing document…", lastAt: now(), done: make(chan struct{})}
}

func (p *ingestionProgressTracker) start() {
	ticker := p.newTicker(ingestionProgressInterval)
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer ticker.Stop()
		for {
			select {
			case <-p.done:
				return
			case <-ticker.C():
				p.publish()
			}
		}
	}()
	p.publish()
}

func (p *ingestionProgressTracker) markMap(processed int, action string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if processed > p.mapProcessed {
		p.mapProcessed = processed
		p.recordProgressLocked(p.mapProcessed)
	}
	if action != "" {
		p.action = action
	}
}

func (p *ingestionProgressTracker) markReduce(processed, entities int, action string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if processed > p.reduceProcessed {
		p.reduceProcessed = processed
		p.recordProgressLocked(p.chaptersTotal + p.reduceProcessed)
	}
	if entities > p.entities {
		p.entities = entities
	}
	if action != "" {
		p.action = action
	}
}

func (p *ingestionProgressTracker) recordProgressLocked(processed int) {
	if processed > p.lastProcessed {
		now := p.now()
		delta := processed - p.lastProcessed
		if delta > 0 {
			sample := now.Sub(p.lastAt).Seconds() / float64(delta)
			if sample >= 0 {
				if p.ewmaSeconds == 0 {
					p.ewmaSeconds = sample
				} else {
					p.ewmaSeconds = .35*sample + .65*p.ewmaSeconds
				}
			}
		}
		p.lastProcessed, p.lastAt = processed, now
	}
}

func (p *ingestionProgressTracker) finish(status string, processed, entities int, action string) {
	p.markReduce(processed, entities, action)
	p.mu.Lock()
	p.status = status
	p.mu.Unlock()
	p.stop()
	p.publish()
}

func (p *ingestionProgressTracker) stop() { p.stopOnce.Do(func() { close(p.done); p.wg.Wait() }) }

func (p *ingestionProgressTracker) snapshot() (string, int, int, int, string, *int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var eta *int
	processed, total := p.mapProcessed, p.chaptersTotal*2
	if p.reduceProcessed > 0 || p.mapProcessed >= p.chaptersTotal {
		processed = p.chaptersTotal + p.reduceProcessed
	}
	if total > 0 && processed*10 >= total && processed < total && p.ewmaSeconds > 0 {
		seconds := int(p.ewmaSeconds*float64(total-processed) + .999)
		eta = &seconds
	}
	return p.status, processed, total, p.entities, p.action, eta
}

func (p *ingestionProgressTracker) publish() {
	status, processed, total, entities, action, eta := p.snapshot()
	p.mu.Lock()
	dbProcessed, dbTotal := p.reduceProcessed, p.chaptersTotal
	p.mu.Unlock()
	p.svc.updateProgress(context.Background(), p.jobID, dbTotal, dbProcessed, entities)
	p.svc.emitProgressDetails(p.jobID, p.userID, status, processed, total, action, eta)
}
