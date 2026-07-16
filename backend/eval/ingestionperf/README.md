# Ingestion performance harness

Run the deterministic plumbing harness from `backend/`:

```bash
go run ./cmd/ingestion-perf -pages 50 -runs 3 -fixture ../artifacts/fixtures/ingestion-50-page.md -output ../artifacts/reports/ingestion-50-page.json
go run ./cmd/ingestion-perf -pages 400 -runs 3 -fixture ../artifacts/fixtures/ingestion-400-page.md -output ../artifacts/reports/ingestion-400-page.json
```

Each JSON artifact records models, rate-limit configuration, fixture words/chunks,
wall samples, p50/p95, throughput, the duplicate natural-key SQL check, and whether
live dependencies ran. Percentiles use nearest-rank, so with the default three runs
p95 is deliberately the slowest recorded run.

`make ingestion-perf` sources `.env` when it exists, so the deterministic report
records the same model and quota configuration that the application would use.

## Live ingestion measurement

`make ingestion-perf-live` builds a first-party binary image from `backend/` and the
versioned fixtures, then runs it beside an isolated PostgreSQL stack. The root
`.dockerignore` excludes `.env` from the image build context; the key exists only as a
runtime environment variable in the benchmark container. The sole host bind is
`artifacts/reports/`, where the JSON report is persisted.

Before writing benchmark data, the command validates `QWEN_API_KEY`, PostgreSQL
connectivity, the `vector` and `age` extensions, and the configured Qwen endpoint. It
runs the actual `IngestionService` asynchronously against an isolated
user/universe/AGE graph, waits for terminal status, and queries duplicate natural
keys. Terminal benchmark data is removed by default; a timed-out job is intentionally
retained to avoid deleting data while its background worker is still active.

```bash
# One live 50-page run: records a measurement but not a percentile claim.
make ingestion-perf-live

# One 400-page run. Run only when its Qwen quota/cost is intentional.
make ingestion-perf-live INGESTION_PERF_PAGES=400

# Three 50-page runs: records nearest-rank p50/p95. This consumes Qwen quota.
make ingestion-perf-live INGESTION_PERF_RUNS=3

# Run both fixture sizes explicitly. This creates a fresh isolated stack per size.
make ingestion-perf-live INGESTION_PERF_PAGES=all
```

`INGESTION_PERF_PAGES` accepts only `50`, `400`, or `all`; an invalid value fails
before Compose starts or Qwen receives a request. `400` never implies a 50-page run.

The live JSON says `measured` only when two or more runs completed with zero
duplicate natural keys. One run preserves its wall time and throughput but leaves
p50/p95 at zero. It deliberately does not claim PF-1/PF-2 passed: compare a
multi-run `p95_seconds` to the sprint thresholds yourself.
These deterministic runs validate fixture/harness plumbing only; they are not evidence
for PF-1/PF-2. For a measured claim, run a live ingest with a validated Qwen key and
Postgres, then execute the SQL stored in `duplicate_natural_key_query`. The live result
must show no rows and the 50/400-page p95 targets in `SPRINT-2-ingestion-performance.md`.
