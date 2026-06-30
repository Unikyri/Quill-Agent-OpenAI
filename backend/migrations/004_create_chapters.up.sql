CREATE TABLE chapters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    work_id UUID NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    title VARCHAR(255),
    order_index INTEGER NOT NULL DEFAULT 1,
    content TEXT DEFAULT '',
    raw_text TEXT DEFAULT '',
    word_count INTEGER DEFAULT 0,
    status VARCHAR(50) DEFAULT 'draft',
    analyzed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_chapters_work_id ON chapters(work_id);
