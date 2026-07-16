package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// QwenRequestClass keeps background ingestion from consuming the capacity
// reserved for live editing and craft interactions.
type QwenRequestClass uint8

const (
	QwenInteractiveRequest QwenRequestClass = iota
	QwenIngestionRequest
)

type qwenRequestClassKey struct{}

func WithQwenRequestClass(ctx context.Context, class QwenRequestClass) context.Context {
	return context.WithValue(ctx, qwenRequestClassKey{}, class)
}

func requestClass(ctx context.Context) QwenRequestClass {
	class, _ := ctx.Value(qwenRequestClassKey{}).(QwenRequestClass)
	return class
}

type qwenModelTier uint8

const (
	qwenTurboTier qwenModelTier = iota
	qwenMaxTier
)

type throttleClock func() time.Time
type throttleSleep func(context.Context, time.Duration) error

type limiterSet struct {
	tokens    *rate.Limiter
	requests  *rate.Limiter
	perSecond *rate.Limiter
}

type throttleTierState struct {
	total     limiterSet
	ingestion limiterSet
}

// QwenThrottle applies the provider quotas before a request is sent. Its
// clock and sleeper are injectable so tests never wait on wall-clock time.
type QwenThrottle struct {
	clock throttleClock
	sleep throttleSleep
	tiers map[qwenModelTier]throttleTierState
	gate  *rampGate
}

func newQwenThrottle(turboTPM, maxTPM, rpm int, reserve float64, maxConcurrency, rampStep int) *QwenThrottle {
	if reserve < 0 || reserve >= 1 {
		reserve = 0.30
	}
	if rpm <= 0 {
		rpm = 600
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 5
	}
	if rampStep <= 0 {
		rampStep = 1
	}
	return &QwenThrottle{
		clock: time.Now,
		sleep: sleepWithContext,
		tiers: map[qwenModelTier]throttleTierState{
			qwenTurboTier: newThrottleTier(turboTPM, rpm, reserve),
			qwenMaxTier:   newThrottleTier(maxTPM, rpm, reserve),
		},
		gate: newRampGate(maxConcurrency, rampStep),
	}
}

func newThrottleTier(tpm, rpm int, reserve float64) throttleTierState {
	if tpm <= 0 {
		tpm = 1
	}
	ingestionScale := 1 - reserve
	return throttleTierState{
		total:     newLimiterSet(tpm, rpm, 10),
		ingestion: newLimiterSet(maxInt(1, int(float64(tpm)*ingestionScale)), maxInt(1, int(float64(rpm)*ingestionScale)), maxInt(1, int(10*ingestionScale))),
	}
}

func newLimiterSet(tpm, rpm, rps int) limiterSet {
	return limiterSet{
		tokens:    rate.NewLimiter(rate.Limit(float64(tpm)/60), maxInt(1, tpm)),
		requests:  rate.NewLimiter(rate.Limit(float64(rpm)/60), maxInt(1, rpm)),
		perSecond: rate.NewLimiter(rate.Limit(rps), rps),
	}
}

func (t *QwenThrottle) acquire(ctx context.Context, tier qwenModelTier, class QwenRequestClass, tokens int) (func(bool), error) {
	if t == nil {
		return func(bool) {}, nil
	}
	if tokens < 1 {
		tokens = 1
	}
	state, ok := t.tiers[tier]
	if !ok {
		return nil, fmt.Errorf("unknown qwen throttle tier")
	}
	// Every request consumes the one provider quota. Ingestion additionally
	// consumes its 70%-of-total allowance, preserving interactive headroom
	// without allowing two independent buckets to exceed the provider quota.
	if err := t.wait(ctx, state.total.requests, 1); err != nil {
		return nil, err
	}
	if err := t.wait(ctx, state.total.perSecond, 1); err != nil {
		return nil, err
	}
	if err := t.wait(ctx, state.total.tokens, tokens); err != nil {
		return nil, err
	}
	if class == QwenIngestionRequest {
		if err := t.wait(ctx, state.ingestion.requests, 1); err != nil {
			return nil, err
		}
		if err := t.wait(ctx, state.ingestion.perSecond, 1); err != nil {
			return nil, err
		}
		if err := t.wait(ctx, state.ingestion.tokens, tokens); err != nil {
			return nil, err
		}
	}
	if err := t.gate.acquire(ctx, t.sleep); err != nil {
		return nil, err
	}
	return t.gate.release, nil
}

func (t *QwenThrottle) wait(ctx context.Context, limiter *rate.Limiter, n int) error {
	for n > 0 {
		chunk := minInt(n, limiter.Burst())
		now := t.clock()
		reservation := limiter.ReserveN(now, chunk)
		if !reservation.OK() {
			return fmt.Errorf("qwen rate quota cannot reserve %d tokens", chunk)
		}
		delay := reservation.DelayFrom(now)
		if delay > 0 {
			if err := t.sleep(ctx, delay); err != nil {
				reservation.CancelAt(t.clock())
				return err
			}
		}
		n -= chunk
	}
	return nil
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type rampGate struct {
	mu       sync.Mutex
	active   int
	limit    int
	max      int
	rampStep int
}

func newRampGate(max, step int) *rampGate {
	return &rampGate{limit: minInt(2, max), max: max, rampStep: step}
}

func (g *rampGate) acquire(ctx context.Context, sleep throttleSleep) error {
	for {
		g.mu.Lock()
		if g.active < g.limit {
			g.active++
			g.mu.Unlock()
			return nil
		}
		g.mu.Unlock()
		if err := sleep(ctx, time.Millisecond); err != nil {
			return err
		}
	}
}

func (g *rampGate) release(success bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.active > 0 {
		g.active--
	}
	if success && g.limit < g.max {
		g.limit = minInt(g.max, g.limit+g.rampStep)
	}
}

func (g *rampGate) currentLimit() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.limit
}

func (g *rampGate) maxLimit() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.max
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
