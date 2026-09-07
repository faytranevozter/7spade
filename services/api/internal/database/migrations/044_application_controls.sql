INSERT INTO feature_settings (key, enabled) VALUES
    ('new_registrations', TRUE),
    ('guest_access', TRUE),
    ('room_creation', TRUE),
    ('quick_play', TRUE)
ON CONFLICT (key) DO NOTHING;
