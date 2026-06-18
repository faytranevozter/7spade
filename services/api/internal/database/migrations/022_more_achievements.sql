-- Hard achievements: win milestones, win streaks, first-place milestones,
-- penalty accumulation, and dedication milestones.

INSERT INTO achievements (id, name, description, icon, display_order, enabled)
VALUES
    ('wins_50',            'Champion',      'Win 50 games',                          '🥇', 90,  TRUE),
    ('wins_100',           'Legend',         'Win 100 games',                         '👑', 100, TRUE),
    ('streak_10',          'Legendary',      'Win 10 games in a row',                 '🌪️', 110, TRUE),
    ('streak_15',          'Mythical',       'Win 15 games in a row',                 '🐉', 120, TRUE),
    ('firsts_50',          'Sovereign',      'Finish 1st in 50 games',                '💎', 130, TRUE),
    ('firsts_100',         'Emperor',        'Finish 1st in 100 games',               '🏆', 140, TRUE),
    ('zero_penalty_games_10', 'Zen Master',  'Finish 10 games with zero penalty',     '🧘', 150, TRUE),
    ('games_200',          'Lifetimer',      'Play 200 games',                        '🗻', 160, TRUE),
    ('human_only_25',      'Social Butterfly', 'Play 25 human-only games',            '🦋', 170, TRUE)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    icon = EXCLUDED.icon,
    display_order = EXCLUDED.display_order,
    enabled = EXCLUDED.enabled,
    updated_at = NOW();

DELETE FROM achievement_rules
WHERE achievement_id IN (
    'wins_50', 'wins_100', 'streak_10', 'streak_15',
    'firsts_50', 'firsts_100', 'zero_penalty_games_10',
    'games_200', 'human_only_25'
);

INSERT INTO achievement_rules (achievement_id, metric, operator, value)
VALUES
    ('wins_50',            'wins',              'gte', '50'),
    ('wins_100',           'wins',              'gte', '100'),
    ('streak_10',          'current_streak',    'gte', '10'),
    ('streak_15',          'current_streak',    'gte', '15'),
    ('firsts_50',          'first_place_count', 'gte', '50'),
    ('firsts_100',         'first_place_count', 'gte', '100'),
    ('zero_penalty_games_10', 'zero_penalty_games', 'gte', '10'),
    ('games_200',          'games_played',      'gte', '200'),
    ('human_only_25',      'human_only_games',  'gte', '25');
