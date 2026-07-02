-- Seed demo user and template universe
INSERT INTO users (id, email, password_hash, display_name, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'demo@quill.ai',
    '$2a$12$S0AJJxJbSguEh6wjh66IFOchI3klGxKghe7C0IsSRk.vCgOCS0.dO',
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
