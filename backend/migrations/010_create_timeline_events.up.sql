CREATE TABLE timeline_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    universe_id UUID NOT NULL REFERENCES universes(id) ON DELETE CASCADE,
    event_entity_id UUID REFERENCES entities(id),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    timeline_position FLOAT,
    timeline_label VARCHAR(255),
    chapter_id UUID REFERENCES chapters(id),
    participants UUID[],
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_timeline_events_universe_id ON timeline_events(universe_id);
