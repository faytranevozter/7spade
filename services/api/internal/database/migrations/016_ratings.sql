-- Skill rating (ELO) on lifetime stats. season_user_stats already carries a
-- per-season rating (migration 015). Default 1200 matches the issue's AC.
-- Existing rows backfill to the default so every recorded player starts level.
ALTER TABLE user_stats ADD COLUMN IF NOT EXISTS rating INTEGER NOT NULL DEFAULT 1200;
