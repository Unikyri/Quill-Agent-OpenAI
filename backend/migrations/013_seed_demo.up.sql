-- Seed demo user and template universe
INSERT INTO users (id, email, password_hash, display_name, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'demo@quill.ai',
    '$2a$12$LJ3m4ys3Lz0QqVqBvQz5YOXZKQxH5tXVYR5Z6b7c8d9e0f1g2h3i4j',
    'Demo Writer',
    NOW(),
    NOW()
) ON CONFLICT (email) DO NOTHING;

INSERT INTO universes (id, user_id, name, description, genre, format, is_demo_template, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000002',
    '00000000-0000-0000-0000-000000000001',
    'Echoes of Eternity',
    'A fantasy saga about the struggle for control of ancestral magic across multiple realms.',
    'fantasy',
    'novel',
    TRUE,
    NOW(),
    NOW()
) ON CONFLICT DO NOTHING;
