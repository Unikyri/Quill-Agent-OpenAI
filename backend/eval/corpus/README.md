# Saga Gold Corpus

This directory contains the manually annotated gold corpus for the "Echoes of Eternity" demo saga that is seeded by migration `014_seed_demo_saga`.

## Files

- `saga_gold.json` — query relevance judgments, forgetting-timeline labels, and consolidation targets.

## Methodology

### Query design

Each query is designed to test a distinct narrative recall connection that a well-performing memory system should retrieve:

- **Secret allegiance** — Kael Drystan's hidden membership in the Hollow Chorus.
- **Imprisonment location** — Seraphine Vane being held in Hollow Reach.
- **Causal responsibility** — the Crown causing the Sundering.
- **Mentorship** — Oracle Iyana as Lyra Vane's mentor.
- **Operational base** — Maven Voss working out of the Echo Spire.
- **Prophecy dependency** — the Convergence depending on the First Echo.

Relevance judgments are expressed as entity names and resolved to UUIDs at test time against the saga universe (`00000000-0000-0000-0000-000000000002`).

### Forgetting timeline

Labels are anchored at the end of Book I (three chapters). They encode the expected survival of entities in working memory:

- `should_be_archived` — entities whose recency and centrality have dropped; they should be candidates for archival.
- `must_stay_active` — entities that remain highly relevant and must stay in active recall.

### Consolidation targets

Entities that appear repeatedly across the narrative and carry rich, summarizable context. These are the intended beneficiaries of the consolidated-memory pipeline.
