CREATE TABLE ingestion_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    universe_id UUID NOT NULL REFERENCES universes(id) ON DELETE CASCADE,
    work_id UUID NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    filename VARCHAR(255),
    file_type VARCHAR(50),
    status VARCHAR(50) DEFAULT 'pending',
    total_chapters_detected INTEGER DEFAULT 0,
    chapters_processed INTEGER DEFAULT 0,
    entities_extracted INTEGER DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_ingestion_jobs_work_id ON ingestion_jobs(work_id);
