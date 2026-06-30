CREATE TABLE plot_holes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    universe_id UUID NOT NULL REFERENCES universes(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    related_entity_ids UUID[],
    first_mentioned_chapter_id UUID REFERENCES chapters(id),
    status VARCHAR(50) DEFAULT 'open',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_plot_holes_universe_id ON plot_holes(universe_id);
