-- Per-user earned achievements. Composite PK makes awarding idempotent.
CREATE TABLE IF NOT EXISTS user_achievements (
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    achievement_id TEXT NOT NULL,
    earned_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, achievement_id)
);

CREATE INDEX IF NOT EXISTS idx_user_achievements_user_id ON user_achievements(user_id);

-- Win-streak counter, maintained transactionally in UpsertUserStats: incremented
-- on a win, reset to 0 on a loss. Backs the streak_3 / streak_5 achievements.
ALTER TABLE user_stats ADD COLUMN IF NOT EXISTS current_streak INTEGER NOT NULL DEFAULT 0;
