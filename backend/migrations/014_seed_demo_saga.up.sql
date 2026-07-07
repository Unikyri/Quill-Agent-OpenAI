-- Migration 014: Seed demo saga "Echoes of Eternity"
-- Populates template universe with 3 works, 3 chapters, 21+ entities,
-- mentions, embeddings, 6+ contradictions, timeline events, plot holes, and AGE graph.
-- All INSERTs use WHERE NOT EXISTS for idempotency with fixed UUIDs.
-- Template universe: 00000000-0000-0000-0000-000000000002

-- ============================================================================
-- Works (3 works — Books I-III)
-- ============================================================================

INSERT INTO works (id, universe_id, title, type, order_index, synopsis, status, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000010',
       '00000000-0000-0000-0000-000000000002',
       'Echoes of Eternity — Book I: The Awakening',
       'novel',
       1,
       'Lyra Vane, a young cartographer, discovers she carries the Echo — an ancestral magic thought lost for three centuries. As the Veil between realms thins, she must master her power before the Hollow Chorus claims the Sundered Isles.',
       'completed',
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM works WHERE id = '00000000-0000-0000-0000-000000000010');

INSERT INTO works (id, universe_id, title, type, order_index, synopsis, status, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000011',
       '00000000-0000-0000-0000-000000000002',
       'Echoes of Eternity — Book II: Fractured Bonds',
       'novel',
       2,
       'With the Veil collapsing, Lyra must forge an uneasy alliance between the Echo Wardens and the Crown. Kael Drystan''s hidden allegiance to the Hollow Chorus threatens to tear the fragile coalition apart.',
       'draft',
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM works WHERE id = '00000000-0000-0000-0000-000000000011');

INSERT INTO works (id, universe_id, title, type, order_index, synopsis, status, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000012',
       '00000000-0000-0000-0000-000000000002',
       'Echoes of Eternity — Book III: The Convergence',
       'novel',
       3,
       'The Sundering approaches its final cycle. Lyra, now the Oracle''s heir, must choose between sealing the Veil forever or letting the Echo flow freely through all three realms — a choice the Crown will kill to prevent.',
       'draft',
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM works WHERE id = '00000000-0000-0000-0000-000000000012');

-- ============================================================================
-- Chapters (3 chapters in Book I with narrative prose)
-- ============================================================================

INSERT INTO chapters (id, work_id, title, order_index, content, raw_text, word_count, status, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000020',
       '00000000-0000-0000-0000-000000000010',
       'Chapter 1: The Echo''s Call',
       1,
       E'The morning mist clung to the Veiled City''s quartz spires as Lyra Vane unrolled her cartography scroll across the worn oak table. The Echo Spire hummed faintly in the distance, a sound most citizens had long stopped noticing.\n\n"Another map of the Sundered Isles?" Kael Drystan leaned against the doorframe, his Crown insignia catching the pale light. "The Wardens already have twelve."\n\nLyra did not look up. "Thirteen, if I finish this. The resonance patterns shifted again last night."\n\nShe traced her finger along the western archipelago, where thin lines marked the Veil''s boundary — the invisible membrane between the physical realm and the Echo. Three hundred years ago, the Sundering had torn that membrane apart. The Echo Wardens, founded by the survivors, had spent every generation since trying to map its scars.\n\nKael moved closer, his boots silent on the stone floor. "Marshal Eron wants you at the Hollow Reach outpost by sunset. Says the Chorus left something behind."\n\n"The Hollow Chorus does not leave things behind," Lyra said. "They take them."\n\nShe remembered her mother, Seraphine Vane, teaching her the old rhymes: *The Chorus sings in hollow bones / The Echo answers, overthrows / But what is taken, what is lost / Returns at last in winter frost.*\n\nHer mother had died seven winters ago, swallowed by the Chasm of Whispers during an expedition to find the First Echo. Or so the Crown''s report said.\n\n"Sunset, Lyra." Kael tapped the doorframe twice — his old signal — and vanished into the corridor.\n\nLyra stared at her map. The resonance lines had shifted, yes. But they had shifted *toward* the Hollow Reach, not away. Something was waking up.',
       E'The morning mist clung to the Veiled City''s quartz spires as Lyra Vane unrolled her cartography scroll across the worn oak table. The Echo Spire hummed faintly in the distance, a sound most citizens had long stopped noticing.\n\n"Another map of the Sundered Isles?" Kael Drystan leaned against the doorframe, his Crown insignia catching the pale light. "The Wardens already have twelve."\n\nLyra did not look up. "Thirteen, if I finish this. The resonance patterns shifted again last night."\n\nShe traced her finger along the western archipelago, where thin lines marked the Veil''s boundary — the invisible membrane between the physical realm and the Echo. Three hundred years ago, the Sundering had torn that membrane apart. The Echo Wardens, founded by the survivors, had spent every generation since trying to map its scars.\n\nKael moved closer, his boots silent on the stone floor. "Marshal Eron wants you at the Hollow Reach outpost by sunset. Says the Chorus left something behind."\n\n"The Hollow Chorus does not leave things behind," Lyra said. "They take them."\n\nShe remembered her mother, Seraphine Vane, teaching her the old rhymes: The Chorus sings in hollow bones / The Echo answers, overthrows / But what is taken, what is lost / Returns at last in winter frost.\n\nHer mother had died seven winters ago, swallowed by the Chasm of Whispers during an expedition to find the First Echo. Or so the Crown''s report said.\n\n"Sunset, Lyra." Kael tapped the doorframe twice — his old signal — and vanished into the corridor.\n\nLyra stared at her map. The resonance lines had shifted, yes. But they had shifted toward the Hollow Reach, not away. Something was waking up.',
       398,
       'draft',
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM chapters WHERE id = '00000000-0000-0000-0000-000000000020');

INSERT INTO chapters (id, work_id, title, order_index, content, raw_text, word_count, status, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000021',
       '00000000-0000-0000-0000-000000000010',
       'Chapter 2: Whispers in the Veil',
       2,
       E'The Hollow Reach outpost perched on the edge of the Chasm of Whispers like a seabird clinging to a cliff. Lyra dismounted, her boots sinking into ash-grey soil that had not seen rain in a generation.\n\nMarshal Eron stood at the chasm''s rim, his silver beard whipping in the wind. "Three hours ago, the Chorus surfaced. Six of them. They left this."\n\nHe handed her a shard of obsidian glass, warm to the touch. Etched into its surface was a map — not of the isles, but of something far older. Ancestral Resonance markers, the kind only the Echo Wardens used before the Sundering.\n\n"This is three hundred twelve years old," Lyra whispered, touching the glass. "See the spiral notation? That''s Warden script from before the schism."\n\n"Before the schism?" Marshal Eron frowned. "The Wardens and the Crown split two hundred years after the Sundering. You''re saying this predates—"\n\n"It predates the Crown," Lyra finished. "The Sundering was three centuries ago. This map was made twelve years before it happened."\n\nThe obsidian pulsed once, and Lyra felt the Echo stir in her chest — a warmth that spread through her ribs like liquid sunlight. It had never done that before.\n\nBehind them, Kael Drystan watched from the shadow of the outpost wall. His hand rested on his sword hilt. But it was not the Marshal he was watching.\n\nIt was the glass.',
       E'The Hollow Reach outpost perched on the edge of the Chasm of Whispers like a seabird clinging to a cliff. Lyra dismounted, her boots sinking into ash-grey soil that had not seen rain in a generation.\n\nMarshal Eron stood at the chasm''s rim, his silver beard whipping in the wind. "Three hours ago, the Chorus surfaced. Six of them. They left this."\n\nHe handed her a shard of obsidian glass, warm to the touch. Etched into its surface was a map — not of the isles, but of something far older. Ancestral Resonance markers, the kind only the Echo Wardens used before the Sundering.\n\n"This is three hundred twelve years old," Lyra whispered, touching the glass. "See the spiral notation? That''s Warden script from before the schism."\n\n"Before the schism?" Marshal Eron frowned. "The Wardens and the Crown split two hundred years after the Sundering. You''re saying this predates—"\n\n"It predates the Crown," Lyra finished. "The Sundering was three centuries ago. This map was made twelve years before it happened."\n\nThe obsidian pulsed once, and Lyra felt the Echo stir in her chest — a warmth that spread through her ribs like liquid sunlight. It had never done that before.\n\nBehind them, Kael Drystan watched from the shadow of the outpost wall. His hand rested on his sword hilt. But it was not the Marshal he was watching.\n\nIt was the glass.',
       367,
       'draft',
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM chapters WHERE id = '00000000-0000-0000-0000-000000000021');

INSERT INTO chapters (id, work_id, title, order_index, content, raw_text, word_count, status, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000022',
       '00000000-0000-0000-0000-000000000010',
       'Chapter 3: The Sundering Dawn',
       3,
       E'Oracle Iyana''s sanctum floated above the Veiled City, suspended by threads of Echo magic older than any living memory. Lyra had visited only once before — the day her mother was declared dead.\n\n"You have the glass," the Oracle said. It was not a question. Her blind eyes, milk-white and unblinking, saw more than any cartographer''s instruments.\n\nLyra placed the obsidian shard on the floating altar. "It shows Ancestral Resonance patterns. Twelve years before the Sundering."\n\n"The First Echo," Iyana breathed. "Your mother found it, child. That is why she did not return."\n\nA cold current passed through the sanctum. Lyra''s hands trembled. "My mother is dead. The Crown confirmed—"\n\n"The Crown confirmed what it wanted you to believe." Iyana turned toward the eastern window, where the first light of dawn was painting the Sundered Isles in gold and crimson. "Seraphine Vane lives. She has been kept in the Hollow Reach''s deepest cell for seven years, her Echo suppressed by Veiled Council decree."\n\nLyra''s blood turned to ice. "Why?"\n\n"Because she knows what really caused the Sundering." The Oracle''s voice dropped to a whisper. "It was not the Hollow Chorus, Lyra. It was the Crown. The First Echo was never lost — it was stolen. And your mother is the key to finding it before the Convergence destroys everything."\n\nOutside, the Echo Spire erupted with light. The Veil, for the first time in three hundred years, began to crack.\n\nAnd somewhere in the depths of the Hollow Reach, Seraphine Vane opened her eyes.',
       E'Oracle Iyana''s sanctum floated above the Veiled City, suspended by threads of Echo magic older than any living memory. Lyra had visited only once before — the day her mother was declared dead.\n\n"You have the glass," the Oracle said. It was not a question. Her blind eyes, milk-white and unblinking, saw more than any cartographer''s instruments.\n\nLyra placed the obsidian shard on the floating altar. "It shows Ancestral Resonance patterns. Twelve years before the Sundering."\n\n"The First Echo," Iyana breathed. "Your mother found it, child. That is why she did not return."\n\nA cold current passed through the sanctum. Lyra''s hands trembled. "My mother is dead. The Crown confirmed—"\n\n"The Crown confirmed what it wanted you to believe." Iyana turned toward the eastern window, where the first light of dawn was painting the Sundered Isles in gold and crimson. "Seraphine Vane lives. She has been kept in the Hollow Reach''s deepest cell for seven years, her Echo suppressed by Veiled Council decree."\n\nLyra''s blood turned to ice. "Why?"\n\n"Because she knows what really caused the Sundering." The Oracle''s voice dropped to a whisper. "It was not the Hollow Chorus, Lyra. It was the Crown. The First Echo was never lost — it was stolen. And your mother is the key to finding it before the Convergence destroys everything."\n\nOutside, the Echo Spire erupted with light. The Veil, for the first time in three hundred years, began to crack.\n\nAnd somewhere in the depths of the Hollow Reach, Seraphine Vane opened her eyes.',
       389,
       'draft',
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM chapters WHERE id = '00000000-0000-0000-0000-000000000022');

-- ============================================================================
-- Entities (21 entities — 6 characters, 5 places, 4 factions, 3 events, 3 concepts)
-- ============================================================================

-- Characters (6)

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, last_mentioned_chapter_id, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000100',
       '00000000-0000-0000-0000-000000000002',
       'character',
       'Lyra Vane',
       ARRAY['Lyra', 'the Cartographer', 'the Oracle''s Heir'],
       'A young cartographer of the Veiled City who discovers she carries the Echo — an ancestral magic thought lost for centuries. Daughter of Seraphine Vane, trained in Warden cartography but never formally inducted.',
       '{"age": 22, "skills": ["cartography", "resonance reading"], "echo_level": "awakening"}',
       'active',
       0.95,
       '00000000-0000-0000-0000-000000000022',
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000100');

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, last_mentioned_chapter_id, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000101',
       '00000000-0000-0000-0000-000000000002',
       'character',
       'Kael Drystan',
       ARRAY['Kael', 'the Blade'],
       'A Crown operative assigned to monitor the Echo Wardens. Sworn to the Crown but secretly affiliated with the Hollow Chorus. Childhood friend of Lyra Vane, torn between duty and conscience.',
       '{"age": 25, "allegiance": "Crown", "secret_allegiance": "Hollow Chorus", "skills": ["swordsmanship", "infiltration"]}',
       'active',
       0.88,
       '00000000-0000-0000-0000-000000000021',
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000101');

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, last_mentioned_chapter_id, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000102',
       '00000000-0000-0000-0000-000000000002',
       'character',
       'Seraphine Vane',
       ARRAY['Seraphine', 'the Lost Cartographer'],
       'Mother of Lyra Vane. A legendary Warden cartographer who discovered the location of the First Echo. Officially declared dead after an expedition to the Chasm of Whispers seven years ago. Secretly imprisoned in the Hollow Reach.',
       '{"age": 47, "status": "imprisoned", "revive_deceased": true, "echo_level": "master", "crime": "discovering the First Echo"}',
       'active',
       0.85,
       '00000000-0000-0000-0000-000000000022',
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000102');

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, last_mentioned_chapter_id, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000103',
       '00000000-0000-0000-0000-000000000002',
       'character',
       'Marshal Eron',
       ARRAY['Eron', 'the Marshal'],
       'Commander of the Crown''s border forces stationed at the Hollow Reach outpost. A veteran of the Chorus skirmishes, pragmatic but fiercely loyal to the Crown. Knows more about the Sundering than he admits.',
       '{"age": 58, "allegiance": "Crown", "rank": "Marshal", "years_of_service": 35}',
       'active',
       0.72,
       '00000000-0000-0000-0000-000000000021',
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000103');

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, last_mentioned_chapter_id, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000104',
       '00000000-0000-0000-0000-000000000002',
       'character',
       'Maven Voss',
       ARRAY['Maven', 'the Archivist'],
       'Chief archivist of the Echo Wardens. A scholar with minimal combat training but unparalleled knowledge of Ancestral Resonance. Guardian of the Warden archives beneath the Echo Spire.',
       '{"age": 34, "role": "archivist", "magical_aptitude": "none", "responsibility": "ancestral records"}',
       'active',
       0.78,
       '00000000-0000-0000-0000-000000000022',
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000104');

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, last_mentioned_chapter_id, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000105',
       '00000000-0000-0000-0000-000000000002',
       'character',
       'Oracle Iyana',
       ARRAY['Iyana', 'the Blind Seer'],
       'The blind Oracle of the Veiled City who perceives the world through Echo resonance. The oldest living being in the three realms. Mentor to Lyra and keeper of the true history of the Sundering.',
       '{"age": "unknown", "abilities": ["echo sight", "timeline perception"], "role": "oracle"}',
       'active',
       0.90,
       '00000000-0000-0000-0000-000000000022',
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000105');

-- Places (5)

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000110',
       '00000000-0000-0000-0000-000000000002',
       'place',
       'Veiled City',
       ARRAY['the City', 'the Veiled Metropolis'],
       'The capital city of the known realm, built beneath the Echo Spire. Quartz architecture channels ambient Echo energy, powering the city and protecting it from Hollow Chorus incursions. Seat of the Crown and home to the Oracle''s floating sanctum.',
       '{"population": 120000, "architecture": "quartz", "government": "Crown monarchy"}',
       'active',
       0.92,
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000110');

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000111',
       '00000000-0000-0000-0000-000000000002',
       'place',
       'Sundered Isles',
       ARRAY['the Isles', 'the Archipelago'],
       'A shattered archipelago in the western sea, formed when the Sundering tore the continent apart three centuries ago. The Veil is thinnest here, and Echo resonance patterns constantly shift. Site of the original Crown betrayal.',
       '{"formation": "the Sundering", "climate": "temperate maritime", "hazard_level": "extreme"}',
       'active',
       0.82,
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000111');

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000112',
       '00000000-0000-0000-0000-000000000002',
       'place',
       'Hollow Reach',
       ARRAY['the Reach', 'the Prison'],
       'A fortified outpost and prison complex on the edge of the Chasm of Whispers. Built by the Crown to monitor Hollow Chorus activity, but its deepest cells hold political prisoners. Seraphine Vane is imprisoned here.',
       '{"type": "fortress-prison", "containment_level": "maximum", "population_description": "guards and prisoners", "secret": "political prison"}',
       'active',
       0.86,
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000112');

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000113',
       '00000000-0000-0000-0000-000000000002',
       'place',
       'Echo Spire',
       ARRAY['the Spire', 'the Beacon'],
       'A towering crystalline structure at the heart of the Veiled City. Channels Echo energy from the Veil into usable power. Its hum is the city''s heartbeat. The Wardens'' archives and Maven Voss''s study are housed in its sublevels.',
       '{"height_meters": 400, "material": "resonance quartz", "function": "echo energy conduit"}',
       'active',
       0.80,
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000113');

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000114',
       '00000000-0000-0000-0000-000000000002',
       'place',
       'Chasm of Whispers',
       ARRAY['the Chasm', 'the Abyss'],
       'A bottomless rift in the earth where the Sundering first broke through. The Hollow Chorus emerges from here. Echo energy is raw and uncontrolled at the rim. Seraphine Vane''s expedition vanished here seven years ago.',
       '{"depth": "unknown", "origin": "Sundering epicenter", "danger": "extreme", "inhabitants": ["Hollow Chorus"]}',
       'active',
       0.84,
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000114');

-- Factions (4)

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000120',
       '00000000-0000-0000-0000-000000000002',
       'faction',
       'Echo Wardens',
       ARRAY['the Wardens', 'Wardens of the Echo'],
       'An ancient order dedicated to mapping and preserving Echo resonance. Founded after the Sundering three hundred years ago. Keepers of Ancestral Resonance knowledge. Rivals of the Crown. Maintain archives beneath the Echo Spire.',
       '{"founded": "post-Sundering", "membership": 400, "headquarters": "Echo Spire", "mission": "preserve Echo knowledge"}',
       'active',
       0.89,
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000120');

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000121',
       '00000000-0000-0000-0000-000000000002',
       'faction',
       'Hollow Chorus',
       ARRAY['the Chorus', 'the Hollow Ones'],
       'A mysterious faction that dwells in the Chasm of Whispers. They wield corrupted Echo magic and seek to tear the Veil permanently open. Their members are said to have no voice of their own — they speak in stolen Echoes.',
       '{"origin": "Chasm of Whispers", "nature": "corrupted echo users", "goal": "tear the Veil", "unique_trait": "steal voices"}',
       'active',
       0.87,
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000121');

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000122',
       '00000000-0000-0000-0000-000000000002',
       'faction',
       'The Crown',
       ARRAY['the Crown Authority', 'the Monarchy'],
       'The ruling government of the Veiled City and surrounding territories. Formed two hundred years after the Sundering from the remnants of the pre-Sundering nobility. Secretly caused the Sundering to seize power over Echo magic.',
       '{"government_type": "monarchy", "founded": "200 years post-Sundering", "secret": "caused the Sundering", "military": "Marshal Eron"}',
       'active',
       0.91,
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000122');

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000123',
       '00000000-0000-0000-0000-000000000002',
       'faction',
       'Veiled Council',
       ARRAY['the Council', 'the Veiled Ones'],
       'A secret council of the Crown''s inner circle that makes the real decisions. They authorized Seraphine Vane''s imprisonment and suppress all knowledge of the Sundering''s true cause. None outside the Crown know they exist.',
       '{"visibility": "secret", "membership_count": 7, "crime": "suppression of Sundering truth", "control": "over Crown decisions"}',
       'active',
       0.75,
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000123');

-- Events (3)

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000130',
       '00000000-0000-0000-0000-000000000002',
       'event',
       'The Sundering',
       ARRAY['the Great Sundering', 'the Cataclysm'],
       'The cataclysmic event three centuries ago that tore the Veil between the physical realm and the Echo. Created the Sundered Isles and the Chasm of Whispers. Official history blames the Hollow Chorus; the truth implicates the Crown.',
       '{"year": "300 years ago", "cause_official": "Hollow Chorus", "cause_actual": "the Crown", "effects": ["Sundered Isles", "Chasm of Whispers", "birth of Echo magic"]}',
       'active',
       0.94,
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000130');

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000131',
       '00000000-0000-0000-0000-000000000002',
       'event',
       'The First Echo',
       ARRAY['the Original Resonance'],
       'The primordial Echo from which all Echo magic descends. Discovered (or stolen) by the Crown before the Sundering. Its location was lost after the cataclysm. Seraphine Vane spent her life searching for it.',
       '{"type": "primordial magic source", "status": "stolen by Crown", "last_known_location": "Sundered Isles", "date": "312 years ago"}',
       'active',
       0.88,
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000131');

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000132',
       '00000000-0000-0000-0000-000000000002',
       'event',
       'The Convergence',
       ARRAY['the Final Convergence'],
       'The prophesied event when the Veil will either be sealed forever or torn completely open, depending on whether the First Echo is returned or destroyed. All three realms will converge at the Sundered Isles.',
       '{"type": "prophecy", "deadline": "imminent", "stakes": "fate of all three realms", "depends_on": "the First Echo"}',
       'active',
       0.83,
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000132');

-- Concepts / World Rules (3)

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000140',
       '00000000-0000-0000-0000-000000000002',
       'world_rule',
       'Echo Magic',
       ARRAY['the Echo', 'Resonance'],
       'The ancestral magic that flows through the Veil between realms. Carriers can sense and manipulate resonance patterns. The Echo responds to emotional intent and can be shaped into physical effects. Corrupted Echo becomes the Hollow Chorus''s weapon.',
       '{"source": "the Veil", "carriers": ["Lyra Vane", "Oracle Iyana", "Hollow Chorus"], "corrupted_form": "Hollow magic"}',
       'active',
       0.93,
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000140');

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000141',
       '00000000-0000-0000-0000-000000000002',
       'world_rule',
       'Ancestral Resonance',
       ARRAY['the Resonance', 'the Old Patterns'],
       'The study and mapping of Echo energy patterns across time. Ancestral Resonance markers are a forgotten Warden script that predates the Sundering. The obsidian shard found at Hollow Reach is the oldest known example.',
       '{"script_type": "pre-Sundering Warden", "oldest_known_example": "obsidian shard", "purpose": "map Echo energy flows"}',
       'active',
       0.79,
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000141');

INSERT INTO entities (id, universe_id, type, name, aliases, description, properties, status, relevance_score, created_at, updated_at)
SELECT '00000000-0000-0000-0000-000000000142',
       '00000000-0000-0000-0000-000000000002',
       'world_rule',
       'The Veil',
       ARRAY['the Membrane', 'the Barrier'],
       'The metaphysical boundary between the physical realm and the Echo. Maintained naturally but weakened by the Sundering. Its health determines the stability of magic across all three realms. The Convergence will decide its fate.',
       '{"nature": "metaphysical barrier", "current_state": "weakening", "thickness_varies_by": "location", "thinnest_at": "Sundered Isles"}',
       'active',
       0.85,
       NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM entities WHERE id = '00000000-0000-0000-0000-000000000142');

-- ============================================================================
-- Entity Mentions (linking entities to chapters — ~15+ mentions)
-- ============================================================================

-- Chapter 1 mentions
INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000200',
       '00000000-0000-0000-0000-000000000100', '00000000-0000-0000-0000-000000000020',
       1, 'Lyra Vane unrolled her cartography scroll', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000200');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000201',
       '00000000-0000-0000-0000-000000000110', '00000000-0000-0000-0000-000000000020',
       1, 'The morning mist clung to the Veiled City''s quartz spires', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000201');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000202',
       '00000000-0000-0000-0000-000000000113', '00000000-0000-0000-0000-000000000020',
       1, 'The Echo Spire hummed faintly in the distance', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000202');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000203',
       '00000000-0000-0000-0000-000000000101', '00000000-0000-0000-0000-000000000020',
       2, 'Kael Drystan leaned against the doorframe', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000203');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000204',
       '00000000-0000-0000-0000-000000000122', '00000000-0000-0000-0000-000000000020',
       2, 'his Crown insignia catching the pale light', 'implicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000204');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000205',
       '00000000-0000-0000-0000-000000000111', '00000000-0000-0000-0000-000000000020',
       3, 'Another map of the Sundered Isles', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000205');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000206',
       '00000000-0000-0000-0000-000000000120', '00000000-0000-0000-0000-000000000020',
       5, 'The Echo Wardens, founded by the survivors', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000206');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000207',
       '00000000-0000-0000-0000-000000000130', '00000000-0000-0000-0000-000000000020',
       5, 'Three hundred years ago, the Sundering had torn that membrane apart', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000207');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000208',
       '00000000-0000-0000-0000-000000000103', '00000000-0000-0000-0000-000000000020',
       6, 'Marshal Eron wants you at the Hollow Reach outpost', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000208');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000209',
       '00000000-0000-0000-0000-000000000112', '00000000-0000-0000-0000-000000000020',
       6, 'want you at the Hollow Reach outpost by sunset', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000209');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000210',
       '00000000-0000-0000-0000-000000000121', '00000000-0000-0000-0000-000000000020',
       8, 'The Hollow Chorus does not leave things behind', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000210');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000211',
       '00000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000020',
       9, 'She remembered her mother, Seraphine Vane, teaching her the old rhymes', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000211');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000212',
       '00000000-0000-0000-0000-000000000114', '00000000-0000-0000-0000-000000000020',
       10, 'swallowed by the Chasm of Whispers during an expedition', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000212');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000213',
       '00000000-0000-0000-0000-000000000131', '00000000-0000-0000-0000-000000000020',
       10, 'expedition to find the First Echo', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000213');

-- Chapter 2 mentions
INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000220',
       '00000000-0000-0000-0000-000000000112', '00000000-0000-0000-0000-000000000021',
       1, 'The Hollow Reach outpost perched on the edge', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000220');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000221',
       '00000000-0000-0000-0000-000000000103', '00000000-0000-0000-0000-000000000021',
       2, 'Marshal Eron stood at the chasm''s rim', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000221');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000222',
       '00000000-0000-0000-0000-000000000121', '00000000-0000-0000-0000-000000000021',
       2, 'the Chorus surfaced. Six of them', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000222');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000223',
       '00000000-0000-0000-0000-000000000141', '00000000-0000-0000-0000-000000000021',
       3, 'Ancestral Resonance markers, the kind only the Echo Wardens used', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000223');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000224',
       '00000000-0000-0000-0000-000000000120', '00000000-0000-0000-0000-000000000021',
       3, 'the Echo Wardens used before the Sundering', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000224');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000225',
       '00000000-0000-0000-0000-000000000130', '00000000-0000-0000-0000-000000000021',
       5, 'The Sundering was three centuries ago', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000225');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000226',
       '00000000-0000-0000-0000-000000000101', '00000000-0000-0000-0000-000000000021',
       7, 'Kael Drystan watched from the shadow of the outpost wall', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000226');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000227',
       '00000000-0000-0000-0000-000000000140', '00000000-0000-0000-0000-000000000021',
       6, 'Lyra felt the Echo stir in her chest', 'implicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000227');

-- Chapter 3 mentions
INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000230',
       '00000000-0000-0000-0000-000000000105', '00000000-0000-0000-0000-000000000022',
       1, 'Oracle Iyana''s sanctum floated above the Veiled City', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000230');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000231',
       '00000000-0000-0000-0000-000000000110', '00000000-0000-0000-0000-000000000022',
       1, 'sanctum floated above the Veiled City', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000231');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000232',
       '00000000-0000-0000-0000-000000000131', '00000000-0000-0000-0000-000000000022',
       4, 'The First Echo. Your mother found it, child', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000232');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000233',
       '00000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000022',
       6, 'Seraphine Vane lives. She has been kept in the Hollow Reach', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000233');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000234',
       '00000000-0000-0000-0000-000000000122', '00000000-0000-0000-0000-000000000022',
       6, 'The Crown confirmed what it wanted you to believe', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000234');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000235',
       '00000000-0000-0000-0000-000000000112', '00000000-0000-0000-0000-000000000022',
       6, 'kept in the Hollow Reach''s deepest cell', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000235');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000236',
       '00000000-0000-0000-0000-000000000123', '00000000-0000-0000-0000-000000000022',
       6, 'her Echo suppressed by Veiled Council decree', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000236');

INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, context_snippet, mention_type, created_at)
SELECT '00000000-0000-0000-0000-000000000237',
       '00000000-0000-0000-0000-000000000132', '00000000-0000-0000-0000-000000000022',
       7, 'before the Convergence destroys everything', 'explicit', NOW()
WHERE NOT EXISTS (SELECT 1 FROM entity_mentions WHERE id = '00000000-0000-0000-0000-000000000237');

-- ============================================================================
-- Entity Embeddings (21 vectors, pseudo-random 1024-dim)
-- Each entity gets a slightly different seed-derived vector for demo distinctiveness.
-- ============================================================================

-- Helper: deterministic 1024-dim vector from an integer seed
DO $$
DECLARE
    eid UUID;
    rec RECORD;
BEGIN
    FOR rec IN SELECT id, (regexp_replace(id::text, '[^0-9]', '', 'g') || '0')::bigint % 1000 AS seed
                FROM entities WHERE universe_id = '00000000-0000-0000-0000-000000000002'
    LOOP
        INSERT INTO entity_embeddings (id, entity_id, description_embedding, created_at, updated_at)
        SELECT md5(rec.id::text || 'embed')::uuid, rec.id,
               -- Generate deterministic pseudo-random 1024-dim vector from seed.
               (SELECT array_agg(
                    ((cos(rec.seed * i * 0.0174533) + 1.0) / 2.0 * 0.9 + 0.05)::real
                )::real[]::vector
                FROM generate_series(1, 1024) i),
               NOW(), NOW()
        WHERE NOT EXISTS (SELECT 1 FROM entity_embeddings WHERE entity_id = rec.id);
    END LOOP;
END $$;

-- ============================================================================
-- Contradictions (6 planted — across 6 types)
-- ============================================================================

-- Contradiction 1: Lyra's mother — dead in Ch1, alive in Ch3 (deceased/alive)
INSERT INTO contradictions (id, universe_id, entity_id, severity, description, suggestion,
    evidence_a, evidence_a_chapter_id, evidence_b, evidence_b_chapter_id,
    fingerprint, status, created_at)
SELECT '00000000-0000-0000-0000-000000000300',
       '00000000-0000-0000-0000-000000000002',
       '00000000-0000-0000-0000-000000000102',
       'high',
       'Seraphine Vane is stated to have died seven winters ago (Ch1), but Oracle Iyana reveals she is alive and imprisoned in Hollow Reach (Ch3). This directly contradicts her presumed death.',
       'Resolve by confirming Seraphine''s survival: the Crown faked her death to suppress her discovery of the First Echo. Mark Ch1 narration as Lyra''s unreliable belief, not objective fact.',
       'Her mother had died seven winters ago, swallowed by the Chasm of Whispers during an expedition to find the First Echo.',
       '00000000-0000-0000-0000-000000000020',
       'Seraphine Vane lives. She has been kept in the Hollow Reach''s deepest cell for seven years.',
       '00000000-0000-0000-0000-000000000022',
       'contra-seraphine-deceased-alive',
       'open', NOW()
WHERE NOT EXISTS (SELECT 1 FROM contradictions WHERE id = '00000000-0000-0000-0000-000000000300');

-- Contradiction 2: Sundering date — 300 years ago in Ch1, 312 in Ch2 (date discrepancy)
INSERT INTO contradictions (id, universe_id, entity_id, severity, description, suggestion,
    evidence_a, evidence_a_chapter_id, evidence_b, evidence_b_chapter_id,
    fingerprint, status, created_at)
SELECT '00000000-0000-0000-0000-000000000301',
       '00000000-0000-0000-0000-000000000002',
       '00000000-0000-0000-0000-000000000130',
       'medium',
       'The Sundering is described as happening "three hundred years ago" (Ch1) but the obsidian shard is dated to "three hundred twelve years" before the Sundering (Ch2), creating a 12-year discrepancy in the timeline.',
       'The 312 figure refers to the creation of the Ancestral Resonance map, not the Sundering itself. The Crown hid the 12-year gap where they secretly experimented with the First Echo. Clarify through Oracle Iyana''s timeline knowledge.',
       'Three hundred years ago, the Sundering had torn that membrane apart.',
       '00000000-0000-0000-0000-000000000020',
       'This is three hundred twelve years old. This map was made twelve years before it happened.',
       '00000000-0000-0000-0000-000000000021',
       'contra-sundering-date-300-vs-312',
       'open', NOW()
WHERE NOT EXISTS (SELECT 1 FROM contradictions WHERE id = '00000000-0000-0000-0000-000000000301');

-- Contradiction 3: Kael's allegiance — sworn to Crown vs. Hollow Chorus (allegiance conflict)
INSERT INTO contradictions (id, universe_id, entity_id, severity, description, suggestion,
    evidence_a, evidence_a_chapter_id, evidence_b, evidence_b_chapter_id,
    fingerprint, status, created_at)
SELECT '00000000-0000-0000-0000-000000000302',
       '00000000-0000-0000-0000-000000000002',
       '00000000-0000-0000-0000-000000000101',
       'high',
       'Kael Drystan is introduced as a Crown operative with insignia (Ch1), but his entity properties list secret_allegiance to the Hollow Chorus, and his behavior in Ch2 (watching Lyra, not the Marshal) suggests divided loyalty.',
       'Intentional character tension: Kael is a double agent. The Crown believes he monitors the Wardens; the Chorus believes he serves them. His true allegiance is unresolved — this should be a character arc, not a continuity error.',
       'Kael Drystan leaned against the doorframe, his Crown insignia catching the pale light.',
       '00000000-0000-0000-0000-000000000020',
       'Kael Drystan watched from the shadow of the outpost wall. His hand rested on his sword hilt. But it was not the Marshal he was watching.',
       '00000000-0000-0000-0000-000000000021',
       'contra-kael-allegiance-crown-vs-chorus',
       'open', NOW()
WHERE NOT EXISTS (SELECT 1 FROM contradictions WHERE id = '00000000-0000-0000-0000-000000000302');

-- Contradiction 4: Maven's magical aptitude — none shown in Ch1 vs. master-level in Ch2 (ability inconsistency)
INSERT INTO contradictions (id, universe_id, entity_id, severity, description, suggestion,
    evidence_a, evidence_a_chapter_id, evidence_b, evidence_b_chapter_id,
    fingerprint, status, created_at)
SELECT '00000000-0000-0000-0000-000000000303',
       '00000000-0000-0000-0000-000000000002',
       '00000000-0000-0000-0000-000000000104',
       'medium',
       'Maven Voss is described as having "minimal combat training" and "magical_aptitude: none" in entity properties, yet Ch2 describes Maven "unraveling the Resonance patterns with master-level precision" when deciphering the obsidian shard.',
       'Maven''s aptitude is non-combat Echo manipulation: she cannot fight, but she can read Resonance patterns at a scholarly level. Clarify in entity description that magical_aptitude refers to combat magic, not scholarly Echo reading.',
       'Maven Voss is introduced as chief archivist with "magical_aptitude: none" per entity properties.',
       '00000000-0000-0000-0000-000000000020',
       'The obsidian pulsed once under Maven''s touch, its Resonance patterns unraveling with master-level precision.',
       '00000000-0000-0000-0000-000000000021',
       'contra-maven-aptitude-none-vs-master',
       'open', NOW()
WHERE NOT EXISTS (SELECT 1 FROM contradictions WHERE id = '00000000-0000-0000-0000-000000000303');

-- Contradiction 5: First Echo timeline — 312 years old in Ch2, but Sundering was only 300 years ago (timeline date conflict)
INSERT INTO contradictions (id, universe_id, entity_id, severity, description, suggestion,
    evidence_a, evidence_a_chapter_id, evidence_b, evidence_b_chapter_id,
    fingerprint, status, created_at)
SELECT '00000000-0000-0000-0000-000000000304',
       '00000000-0000-0000-0000-000000000002',
       '00000000-0000-0000-0000-000000000131',
       'medium',
       'The First Echo entity lists its date as "312 years ago", but the Sundering happened only 300 years ago. If the First Echo predates the Sundering by 12 years, the Crown had the Echo before the cataclysm — contradicting the narrative that the Sundering released Echo magic into the world.',
       'Intentional reveal: the Crown possessed the First Echo for 12 years before the Sundering. The cataclysm was caused by the Crown''s failed attempt to weaponize it. This is the central secret Oracle Iyana guards.',
       'The First Echo is listed as occurring "312 years ago" per entity properties.',
       '00000000-0000-0000-0000-000000000020',
       'The Sundering was three centuries ago. This map was made twelve years before it happened.',
       '00000000-0000-0000-0000-000000000021',
       'contra-first-echo-312-vs-sundering-300',
       'open', NOW()
WHERE NOT EXISTS (SELECT 1 FROM contradictions WHERE id = '00000000-0000-0000-0000-000000000304');

-- Contradiction 6: Hollow Reach description — ash-grey soil "no rain in a generation" vs. temperate maritime climate (place inconsistency)
INSERT INTO contradictions (id, universe_id, entity_id, severity, description, suggestion,
    evidence_a, evidence_a_chapter_id, evidence_b, evidence_b_chapter_id,
    fingerprint, status, created_at)
SELECT '00000000-0000-0000-0000-000000000305',
       '00000000-0000-0000-0000-000000000002',
       '00000000-0000-0000-0000-000000000112',
       'low',
       'The Hollow Reach outpost is described with "ash-grey soil that had not seen rain in a generation" (Ch2), yet the Sundered Isles entity lists the region''s climate as "temperate maritime" — typically rainy and mild. A temperate maritime zone should not have generation-long droughts.',
       'The Hollow Reach sits at the edge of the Chasm where Echo energy suppresses normal weather patterns. The drought is localized to the Chasm''s rim, not the broader Isles. Add a "microclimate" property to Hollow Reach entity.',
       'Her boots sinking into ash-grey soil that had not seen rain in a generation.',
       '00000000-0000-0000-0000-000000000021',
       'Sundered Isles climate listed as "temperate maritime" — a rainy, mild climate type.',
       '00000000-0000-0000-0000-000000000020',
       'contra-hollow-reach-climate-arid-vs-maritime',
       'open', NOW()
WHERE NOT EXISTS (SELECT 1 FROM contradictions WHERE id = '00000000-0000-0000-0000-000000000305');

-- ============================================================================
-- Timeline Events (8 events spanning the saga timeline)
-- ============================================================================

INSERT INTO timeline_events (id, universe_id, event_entity_id, title, description,
    timeline_position, timeline_label, chapter_id, participants, created_at)
SELECT '00000000-0000-0000-0000-000000000400',
       '00000000-0000-0000-0000-000000000002',
       '00000000-0000-0000-0000-000000000131',
       'The First Echo Discovered',
       'The Crown secretly discovers the First Echo, a primordial source of Echo magic. Its existence is hidden from the Wardens and the public.',
       -312.0, '312 YS', NULL,
       ARRAY['00000000-0000-0000-0000-000000000122']::uuid[],
       NOW()
WHERE NOT EXISTS (SELECT 1 FROM timeline_events WHERE id = '00000000-0000-0000-0000-000000000400');

INSERT INTO timeline_events (id, universe_id, event_entity_id, title, description,
    timeline_position, timeline_label, chapter_id, participants, created_at)
SELECT '00000000-0000-0000-0000-000000000401',
       '00000000-0000-0000-0000-000000000002',
       '00000000-0000-0000-0000-000000000130',
       'The Sundering',
       'The Crown''s failed attempt to weaponize the First Echo tears the Veil open. The Sundered Isles are formed. The Chasm of Whispers opens. Echo magic floods the world. The Crown blames the Hollow Chorus.',
       -300.0, '300 YS', NULL,
       ARRAY['00000000-0000-0000-0000-000000000122', '00000000-0000-0000-0000-000000000121', '00000000-0000-0000-0000-000000000120']::uuid[],
       NOW()
WHERE NOT EXISTS (SELECT 1 FROM timeline_events WHERE id = '00000000-0000-0000-0000-000000000401');

INSERT INTO timeline_events (id, universe_id, event_entity_id, title, description,
    timeline_position, timeline_label, chapter_id, participants, created_at)
SELECT '00000000-0000-0000-0000-000000000402',
       '00000000-0000-0000-0000-000000000002',
       '00000000-0000-0000-0000-000000000120',
       'Echo Wardens Founded',
       'Survivors of the Sundering band together to study and map Echo resonance patterns. The Wardens establish their archives beneath the Echo Spire.',
       -290.0, '290 YS', NULL,
       ARRAY['00000000-0000-0000-0000-000000000120']::uuid[],
       NOW()
WHERE NOT EXISTS (SELECT 1 FROM timeline_events WHERE id = '00000000-0000-0000-0000-000000000402');

INSERT INTO timeline_events (id, universe_id, event_entity_id, title, description,
    timeline_position, timeline_label, chapter_id, participants, created_at)
SELECT '00000000-0000-0000-0000-000000000403',
       '00000000-0000-0000-0000-000000000002',
       '00000000-0000-0000-0000-000000000122',
       'The Crown Established',
       'The remnants of pre-Sundering nobility consolidate power in the Veiled City, forming the Crown. They secretly maintain control over the First Echo''s suppressed history.',
       -100.0, '100 YS', NULL,
       ARRAY['00000000-0000-0000-0000-000000000122', '00000000-0000-0000-0000-000000000123']::uuid[],
       NOW()
WHERE NOT EXISTS (SELECT 1 FROM timeline_events WHERE id = '00000000-0000-0000-0000-000000000403');

INSERT INTO timeline_events (id, universe_id, event_entity_id, title, description,
    timeline_position, timeline_label, chapter_id, participants, created_at)
SELECT '00000000-0000-0000-0000-000000000404',
       '00000000-0000-0000-0000-000000000002',
       '00000000-0000-0000-0000-000000000102',
       'Seraphine Vane''s Expedition',
       'Seraphine Vane leads an expedition to the Chasm of Whispers and discovers the location of the First Echo. The Crown captures her and fakes her death. She is imprisoned in the Hollow Reach.',
       -7.0, '7 YS', NULL,
       ARRAY['00000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000122', '00000000-0000-0000-0000-000000000123']::uuid[],
       NOW()
WHERE NOT EXISTS (SELECT 1 FROM timeline_events WHERE id = '00000000-0000-0000-0000-000000000404');

INSERT INTO timeline_events (id, universe_id, event_entity_id, title, description,
    timeline_position, timeline_label, chapter_id, participants, created_at)
SELECT '00000000-0000-0000-0000-000000000405',
       '00000000-0000-0000-0000-000000000002',
       '00000000-0000-0000-0000-000000000100',
       'Lyra''s Echo Awakens',
       'Lyra Vane discovers she carries the Echo during her cartography work. The resonance patterns shift toward the Hollow Reach. Oracle Iyana begins her mentorship.',
       0.0, 'Present Day', '00000000-0000-0000-0000-000000000020',
       ARRAY['00000000-0000-0000-0000-000000000100', '00000000-0000-0000-0000-000000000105']::uuid[],
       NOW()
WHERE NOT EXISTS (SELECT 1 FROM timeline_events WHERE id = '00000000-0000-0000-0000-000000000405');

INSERT INTO timeline_events (id, universe_id, event_entity_id, title, description,
    timeline_position, timeline_label, chapter_id, participants, created_at)
SELECT '00000000-0000-0000-0000-000000000406',
       '00000000-0000-0000-0000-000000000002',
       '00000000-0000-0000-0000-000000000141',
       'Obsidian Shard Discovery',
       'A Hollow Chorus surfacing at Hollow Reach leaves behind an obsidian shard with Ancestral Resonance markers. Lyra dates it to 312 years ago — twelve years before the Sundering.',
       0.1, 'Present Day +', '00000000-0000-0000-0000-000000000021',
       ARRAY['00000000-0000-0000-0000-000000000100', '00000000-0000-0000-0000-000000000103', '00000000-0000-0000-0000-000000000121']::uuid[],
       NOW()
WHERE NOT EXISTS (SELECT 1 FROM timeline_events WHERE id = '00000000-0000-0000-0000-000000000406');

INSERT INTO timeline_events (id, universe_id, event_entity_id, title, description,
    timeline_position, timeline_label, chapter_id, participants, created_at)
SELECT '00000000-0000-0000-0000-000000000407',
       '00000000-0000-0000-0000-000000000002',
       '00000000-0000-0000-0000-000000000102',
       'Seraphine''s Survival Revealed',
       'Oracle Iyana reveals to Lyra that Seraphine Vane is alive and imprisoned in the Hollow Reach. The Veil begins to crack, signaling the approach of the Convergence.',
       0.2, 'Present Day ++', '00000000-0000-0000-0000-000000000022',
       ARRAY['00000000-0000-0000-0000-000000000100', '00000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000105', '00000000-0000-0000-0000-000000000122', '00000000-0000-0000-0000-000000000123']::uuid[],
       NOW()
WHERE NOT EXISTS (SELECT 1 FROM timeline_events WHERE id = '00000000-0000-0000-0000-000000000407');

-- ============================================================================
-- Plot Holes (4 — unresolved narrative gaps)
-- ============================================================================

INSERT INTO plot_holes (id, universe_id, title, description, related_entity_ids,
    first_mentioned_chapter_id, status, created_at)
SELECT '00000000-0000-0000-0000-000000000500',
       '00000000-0000-0000-0000-000000000002',
       'How did the Crown steal the First Echo?',
       'The First Echo is described as a primordial force of nature, yet the Crown managed to steal and contain it 312 years ago. No explanation is given for how a pre-industrial monarchy could capture, contain, and experiment on a metaphysical energy source. This gap undermines the Crown''s technological plausibility.',
       ARRAY['00000000-0000-0000-0000-000000000122', '00000000-0000-0000-0000-000000000131']::uuid[],
       '00000000-0000-0000-0000-000000000021',
       'open', NOW()
WHERE NOT EXISTS (SELECT 1 FROM plot_holes WHERE id = '00000000-0000-0000-0000-000000000500');

INSERT INTO plot_holes (id, universe_id, title, description, related_entity_ids,
    first_mentioned_chapter_id, status, created_at)
SELECT '00000000-0000-0000-0000-000000000501',
       '00000000-0000-0000-0000-000000000002',
       'Why has the Oracle not acted sooner?',
       'Oracle Iyana knows the Crown''s secret — that they caused the Sundering and imprisoned Seraphine Vane. She has known for seven years. Yet she took no action until Lyra brought her the obsidian shard. Why wait seven years? This inaction seems inexplicable for a being of her power and knowledge.',
       ARRAY['00000000-0000-0000-0000-000000000105', '00000000-0000-0000-0000-000000000122', '00000000-0000-0000-0000-000000000102']::uuid[],
       '00000000-0000-0000-0000-000000000022',
       'open', NOW()
WHERE NOT EXISTS (SELECT 1 FROM plot_holes WHERE id = '00000000-0000-0000-0000-000000000501');

INSERT INTO plot_holes (id, universe_id, title, description, related_entity_ids,
    first_mentioned_chapter_id, status, created_at)
SELECT '00000000-0000-0000-0000-000000000502',
       '00000000-0000-0000-0000-000000000002',
       'The Hollow Chorus purpose',
       'The Hollow Chorus is described as a faction that "seeks to tear the Veil permanently open," but their motivation is never explained. Why do they want the Veil destroyed? What do they gain? They are treated as antagonists but given no coherent goal beyond destruction.',
       ARRAY['00000000-0000-0000-0000-000000000121', '00000000-0000-0000-0000-000000000142']::uuid[],
       '00000000-0000-0000-0000-000000000020',
       'open', NOW()
WHERE NOT EXISTS (SELECT 1 FROM plot_holes WHERE id = '00000000-0000-0000-0000-000000000502');

INSERT INTO plot_holes (id, universe_id, title, description, related_entity_ids,
    first_mentioned_chapter_id, status, created_at)
SELECT '00000000-0000-0000-0000-000000000503',
       '00000000-0000-0000-0000-000000000002',
       'The Convergence mechanics unexplained',
       'The Convergence is described as a prophesied event where "all three realms will converge," but only two realms — the physical realm and the Echo — have been introduced. What is the third realm? How does the Convergence actually work? The prophecy drives the entire plot but its mechanics are never defined.',
       ARRAY['00000000-0000-0000-0000-000000000132', '00000000-0000-0000-0000-000000000142', '00000000-0000-0000-0000-000000000140']::uuid[],
       '00000000-0000-0000-0000-000000000022',
       'open', NOW()
WHERE NOT EXISTS (SELECT 1 FROM plot_holes WHERE id = '00000000-0000-0000-0000-000000000503');

-- ============================================================================
-- AGE Graph — vertices for all entities + edges for relationships
-- ============================================================================

-- Load AGE extension and set search_path for cypher() function
LOAD 'age';
SET search_path = ag_catalog, "$user", public;

-- Create the graph if it doesn't exist
SELECT create_graph('universe_00000000-0000-0000-0000-000000000002');

-- Graph name: universe_{templateUUID}. The graph should already exist from migration 013
-- or will be created on first use. Seed nodes and edges unconditionally via cypher.

-- Create vertices for each entity (skip if already exists)
SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:character {entity_id: '00000000-0000-0000-0000-000000000100', name: 'Lyra Vane', type: 'character'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:character {entity_id: '00000000-0000-0000-0000-000000000101', name: 'Kael Drystan', type: 'character'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:character {entity_id: '00000000-0000-0000-0000-000000000102', name: 'Seraphine Vane', type: 'character'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:character {entity_id: '00000000-0000-0000-0000-000000000103', name: 'Marshal Eron', type: 'character'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:character {entity_id: '00000000-0000-0000-0000-000000000104', name: 'Maven Voss', type: 'character'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:character {entity_id: '00000000-0000-0000-0000-000000000105', name: 'Oracle Iyana', type: 'character'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:place {entity_id: '00000000-0000-0000-0000-000000000110', name: 'Veiled City', type: 'place'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:place {entity_id: '00000000-0000-0000-0000-000000000111', name: 'Sundered Isles', type: 'place'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:place {entity_id: '00000000-0000-0000-0000-000000000112', name: 'Hollow Reach', type: 'place'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:place {entity_id: '00000000-0000-0000-0000-000000000113', name: 'Echo Spire', type: 'place'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:place {entity_id: '00000000-0000-0000-0000-000000000114', name: 'Chasm of Whispers', type: 'place'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:faction {entity_id: '00000000-0000-0000-0000-000000000120', name: 'Echo Wardens', type: 'faction'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:faction {entity_id: '00000000-0000-0000-0000-000000000121', name: 'Hollow Chorus', type: 'faction'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:faction {entity_id: '00000000-0000-0000-0000-000000000122', name: 'The Crown', type: 'faction'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:faction {entity_id: '00000000-0000-0000-0000-000000000123', name: 'Veiled Council', type: 'faction'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:event {entity_id: '00000000-0000-0000-0000-000000000130', name: 'The Sundering', type: 'event'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:event {entity_id: '00000000-0000-0000-0000-000000000131', name: 'The First Echo', type: 'event'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:event {entity_id: '00000000-0000-0000-0000-000000000132', name: 'The Convergence', type: 'event'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:world_rule {entity_id: '00000000-0000-0000-0000-000000000140', name: 'Echo Magic', type: 'world_rule'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:world_rule {entity_id: '00000000-0000-0000-0000-000000000141', name: 'Ancestral Resonance', type: 'world_rule'}) $$)
AS (v agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ CREATE (:world_rule {entity_id: '00000000-0000-0000-0000-000000000142', name: 'The Veil', type: 'world_rule'}) $$)
AS (v agtype);

-- Create edges: relationship types ALLY_OF, MEMBER_OF, LOCATED_IN, OPPOSES, PARENT_OF, MENTOR_OF, CAUSED_BY, CONTROLS, IMPRISONED_IN

-- Lyra relationships
SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000102'}), (b {entity_id: '00000000-0000-0000-0000-000000000100'}) CREATE (a)-[:PARENT_OF]->(b) $$)
AS (r agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000105'}), (b {entity_id: '00000000-0000-0000-0000-000000000100'}) CREATE (a)-[:MENTOR_OF]->(b) $$)
AS (r agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000100'}), (b {entity_id: '00000000-0000-0000-0000-000000000101'}) CREATE (a)-[:ALLY_OF]->(b) $$)
AS (r agtype);

-- Kael relationships
SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000101'}), (b {entity_id: '00000000-0000-0000-0000-000000000122'}) CREATE (a)-[:MEMBER_OF]->(b) $$)
AS (r agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000101'}), (b {entity_id: '00000000-0000-0000-0000-000000000121'}) CREATE (a)-[:MEMBER_OF]->(b) $$)
AS (r agtype);

-- Seraphine relationships
SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000102'}), (b {entity_id: '00000000-0000-0000-0000-000000000120'}) CREATE (a)-[:MEMBER_OF]->(b) $$)
AS (r agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000102'}), (b {entity_id: '00000000-0000-0000-0000-000000000112'}) CREATE (a)-[:IMPRISONED_IN]->(b) $$)
AS (r agtype);

-- Marshal Eron
SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000103'}), (b {entity_id: '00000000-0000-0000-0000-000000000122'}) CREATE (a)-[:MEMBER_OF]->(b) $$)
AS (r agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000103'}), (b {entity_id: '00000000-0000-0000-0000-000000000112'}) CREATE (a)-[:LOCATED_IN]->(b) $$)
AS (r agtype);

-- Maven Voss
SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000104'}), (b {entity_id: '00000000-0000-0000-0000-000000000120'}) CREATE (a)-[:MEMBER_OF]->(b) $$)
AS (r agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000104'}), (b {entity_id: '00000000-0000-0000-0000-000000000113'}) CREATE (a)-[:LOCATED_IN]->(b) $$)
AS (r agtype);

-- Oracle Iyana
SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000105'}), (b {entity_id: '00000000-0000-0000-0000-000000000110'}) CREATE (a)-[:LOCATED_IN]->(b) $$)
AS (r agtype);

-- Place relationships
SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000113'}), (b {entity_id: '00000000-0000-0000-0000-000000000110'}) CREATE (a)-[:LOCATED_IN]->(b) $$)
AS (r agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000112'}), (b {entity_id: '00000000-0000-0000-0000-000000000114'}) CREATE (a)-[:LOCATED_IN]->(b) $$)
AS (r agtype);

-- Faction relationships
SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000120'}), (b {entity_id: '00000000-0000-0000-0000-000000000122'}) CREATE (a)-[:OPPOSES]->(b) $$)
AS (r agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000121'}), (b {entity_id: '00000000-0000-0000-0000-000000000122'}) CREATE (a)-[:OPPOSES]->(b) $$)
AS (r agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000121'}), (b {entity_id: '00000000-0000-0000-0000-000000000120'}) CREATE (a)-[:OPPOSES]->(b) $$)
AS (r agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000123'}), (b {entity_id: '00000000-0000-0000-0000-000000000122'}) CREATE (a)-[:CONTROLS]->(b) $$)
AS (r agtype);

-- Event relationships
SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000130'}), (b {entity_id: '00000000-0000-0000-0000-000000000122'}) CREATE (a)-[:CAUSED_BY]->(b) $$)
AS (r agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000131'}), (b {entity_id: '00000000-0000-0000-0000-000000000122'}) CREATE (a)-[:CONTROLLED_BY]->(b) $$)
AS (r agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000132'}), (b {entity_id: '00000000-0000-0000-0000-000000000131'}) CREATE (a)-[:DEPENDS_ON]->(b) $$)
AS (r agtype);

-- World rule relationships
SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000140'}), (b {entity_id: '00000000-0000-0000-0000-000000000142'}) CREATE (a)-[:FLOWS_THROUGH]->(b) $$)
AS (r agtype);

SELECT * FROM cypher('universe_00000000-0000-0000-0000-000000000002',
    $$ MATCH (a {entity_id: '00000000-0000-0000-0000-000000000141'}), (b {entity_id: '00000000-0000-0000-0000-000000000140'}) CREATE (a)-[:MAPS]->(b) $$)
AS (r agtype);

-- ponytail: reset search_path so later migrations applied over the same
-- session/connection (test harness runs all migration files on one shared
-- conn) don't create their tables inside ag_catalog instead of public.
RESET search_path;
