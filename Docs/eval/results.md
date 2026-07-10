# Memory Recall Evaluation Report

Generated: 2026-07-08T03:56:46Z

| Query | recall@5 | precision@5 | MRR | nDCG@5 |
|-------|----------|-------------|-----|--------|
| Who secretly serves the Hollow Chorus? | 0.000 | 0.000 | 0.167 | 0.000 |
| Where is Seraphine Vane imprisoned? | 0.000 | 0.000 | 0.167 | 0.000 |
| Which faction caused the Sundering? | 0.500 | 0.200 | 1.000 | 0.613 |
| Who is Lyra Vane's mentor? | 1.000 | 0.200 | 0.250 | 0.431 |
| Where does Maven Voss operate from? | 0.000 | 0.000 | 0.125 | 0.000 |
| What is the Convergence dependent on? | 0.000 | 0.000 | 0.111 | 0.000 |

## Latency Benchmark

Degraded-mode `RecallWithQuery` latency (nil embedding, k=5).

| Entities | p50 (ms) | p95 (ms) |
|----------|----------|----------|
| 50 | 2.000 | 3.000 |
| 200 | 2.000 | 3.000 |
| 1000 | 5.000 | 6.000 |
| 5000 | 19.000 | 23.000 |

## Forgetting Timeline

After 17 decay ticks (lambda=0.10, threshold=0.15).

| Metric | Value |
|--------|-------|
| Total active entities | 15 |
| Archived entities | 6 |
| Should be archived (gold) | 2 |
| Must stay active (gold) | 3 |
| False negatives (should archive, stayed active) | 0 |
| False positives (should stay active, archived) | 0 |
