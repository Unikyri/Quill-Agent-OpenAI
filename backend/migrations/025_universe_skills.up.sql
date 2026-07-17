-- Per-universe activation of the immutable editorial skill registry.
CREATE TABLE universe_skills (
    universe_id UUID NOT NULL REFERENCES universes(id) ON DELETE CASCADE,
    skill_name TEXT NOT NULL,
    activated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (universe_id, skill_name)
);

CREATE INDEX idx_universe_skills_universe_activated
    ON universe_skills (universe_id, activated_at, skill_name);
