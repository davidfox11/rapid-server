INSERT INTO categories (id, name, slug, description, question_count) VALUES
('c0000000-0000-0000-0000-000000000001', 'General Knowledge', 'general-knowledge', 'A broad mix of trivia covering science, culture, language, and everyday facts.', 50),
('c0000000-0000-0000-0000-000000000002', 'Movies', 'movies', 'Classic and modern cinema — directors, actors, quotes, and box office hits.', 50),
('c0000000-0000-0000-0000-000000000003', 'History', 'history', 'World history from ancient civilizations to modern events.', 50),
('c0000000-0000-0000-0000-000000000004', 'Geography', 'geography', 'Countries, capitals, landmarks, rivers, and maps.', 50),
('c0000000-0000-0000-0000-000000000005', 'Ireland', 'ireland', 'Irish history, culture, GAA, Gaeilge, music, and places.', 50)
ON CONFLICT (id) DO NOTHING;
