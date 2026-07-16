package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/quill/backend/internal/config"
)

type fakeThrottleTime struct {
	now    time.Time
	sleeps []time.Duration
}

func (f *fakeThrottleTime) clock() time.Time { return f.now }
func (f *fakeThrottleTime) sleep(_ context.Context, delay time.Duration) error {
	f.sleeps = append(f.sleeps, delay)
	f.now = f.now.Add(delay)
	return nil
}

func deterministicThrottle(t *testing.T, tpm, rpm int, reserve float64) (*QwenThrottle, *fakeThrottleTime) {
	t.Helper()
	fake := &fakeThrottleTime{now: time.Unix(0, 0)}
	throttle := newQwenThrottle(tpm, tpm, rpm, reserve, 2, 1)
	throttle.clock = fake.clock
	throttle.sleep = fake.sleep
	return throttle, fake
}

func acquireAndRelease(t *testing.T, throttle *QwenThrottle, ctx context.Context, tokens int) {
	t.Helper()
	release, err := throttle.acquire(ctx, qwenTurboTier, requestClass(ctx), tokens)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	release(true)
}

func TestQwenThrottleSharesProviderQuotaAcrossClasses(t *testing.T) {
	throttle, fake := deterministicThrottle(t, 4, 100, 0.5)
	ctx := context.Background()

	acquireAndRelease(t, throttle, WithQwenRequestClass(ctx, QwenIngestionRequest), 4)
	acquireAndRelease(t, throttle, ctx, 1)

	if len(fake.sleeps) == 0 {
		t.Fatal("interactive request did not wait after ingestion exhausted the shared provider TPM quota")
	}
}

func TestQwenThrottleKeepsInteractiveReserveDuringIngestion(t *testing.T) {
	throttle, fake := deterministicThrottle(t, 100, 100, 0.30)
	ingestion := WithQwenRequestClass(context.Background(), QwenIngestionRequest)

	for range 7 {
		acquireAndRelease(t, throttle, ingestion, 10)
	}
	if got := len(fake.sleeps); got != 0 {
		t.Fatalf("ingestion unexpectedly waited before using its 70-token allowance: %d sleeps", got)
	}
	acquireAndRelease(t, throttle, context.Background(), 30)
	if got := len(fake.sleeps); got != 0 {
		t.Fatalf("interactive request could not use the 30-token reserve: %d sleeps", got)
	}
	acquireAndRelease(t, throttle, ingestion, 1)
	if len(fake.sleeps) == 0 {
		t.Fatal("ingestion exceeded its reserved 70% allowance without waiting")
	}
}

func TestQwenFallbackUsesFallbackModelTier(t *testing.T) {
	var models []string
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request QwenRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		mu.Lock()
		models = append(models, request.Model)
		count := len(models)
		mu.Unlock()
		if count == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	svc := NewQwenService(&config.Config{
		QwenBaseURL: server.URL, QwenAPIKey: "test", QwenMaxConcurrency: 1, QwenTurboConcurrency: 1,
		QwenExtractionModel: "extract", QwenReasoningModel: "reason", QwenFallbackModel: "reason", QwenFallbackOn429: true,
		QwenRetryMaxAttempts: 1, LLMTPMTurbo: 5, LLMTPMMax: 9, LLMRPM: 100, LLMInteractiveReserve: 0.30,
	}, nil)
	fake := &fakeThrottleTime{now: time.Unix(0, 0)}
	svc.throttle.clock, svc.throttle.sleep = fake.clock, fake.sleep
	svc.retrySleep = fake.sleep
	svc.jitter = func(time.Duration) time.Duration { return 0 }

	resp, release, err := svc.sendQwenRequest(context.Background(), qwenTurboTier, "extract", http.MethodPost, "/chat/completions", QwenRequest{Model: "extract"}, 1, true)
	if err != nil {
		t.Fatalf("sendQwenRequest: %v", err)
	}
	resp.Body.Close()
	release(true)

	mu.Lock()
	gotModels := append([]string(nil), models...)
	mu.Unlock()
	if len(gotModels) != 2 || gotModels[0] != "extract" || gotModels[1] != "reason" {
		t.Fatalf("models = %v, want [extract reason]", gotModels)
	}
	if got := svc.throttle.tiers[qwenTurboTier].total.tokens.TokensAt(fake.now); got != 4 {
		t.Fatalf("turbo tokens = %.0f, want 4 after primary 429", got)
	}
	if got := svc.throttle.tiers[qwenMaxTier].total.tokens.TokensAt(fake.now); got != 8 {
		t.Fatalf("max tokens = %.0f, want 8 after fallback", got)
	}
}

func TestGenerateEmbeddingUsesCentralThrottle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.1]}]}`))
	}))
	defer server.Close()
	svc := NewQwenService(&config.Config{QwenBaseURL: server.URL, QwenAPIKey: "test", QwenMaxConcurrency: 1, QwenTurboConcurrency: 1, QwenEmbeddingModel: "embed", LLMTPMTurbo: 10, LLMTPMMax: 10, LLMRPM: 100, LLMInteractiveReserve: 0.30}, nil)
	fake := &fakeThrottleTime{now: time.Unix(0, 0)}
	svc.throttle.clock, svc.throttle.sleep = fake.clock, fake.sleep
	if _, err := svc.GenerateEmbedding(context.Background(), "text"); err != nil {
		t.Fatalf("GenerateEmbedding: %v", err)
	}
	if got := svc.throttle.tiers[qwenTurboTier].total.tokens.TokensAt(fake.now); got >= 10 {
		t.Fatalf("embedding bypassed central throttle; remaining tokens %.0f", got)
	}
}
