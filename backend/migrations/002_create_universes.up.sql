CREATE TABLE universes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    genre VARCHAR(50),
    format VARCHAR(50) NOT NULL,
    session_id VARCHAR(255),
    is_demo_template BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_universes_user_id ON universes(user_id);
CREATE INDEX idx_universes_session_id ON universes(session_id);
