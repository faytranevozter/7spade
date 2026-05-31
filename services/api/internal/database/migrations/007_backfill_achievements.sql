-- One-time backfill of data-derivable achievements for players who already had
-- recorded games before the achievements feature shipped. Without this, the
-- runtime evaluator (which awards only the single highest milestone tier crossed
-- per save) would skip lower tiers for pre-existing players and timestamp the
-- one tier they do cross with a "now" far later than when they earned it.
--
-- Streak achievements (streak_3 / streak_5) are intentionally NOT backfilled:
-- they are order-dependent and would require replaying each player's games
-- chronologically, which is out of scope (see docs/specs/achievements.md ss.10).
-- earned_at defaults to NOW() since the true earn time is unknown for backfill.

-- Games-played milestones, derived from the denormalized counter.
INSERT INTO user_achievements (user_id, achievement_id)
SELECT user_id, 'games_10' FROM user_stats WHERE games_played >= 10
ON CONFLICT (user_id, achievement_id) DO NOTHING;

INSERT INTO user_achievements (user_id, achievement_id)
SELECT user_id, 'games_50' FROM user_stats WHERE games_played >= 50
ON CONFLICT (user_id, achievement_id) DO NOTHING;

INSERT INTO user_achievements (user_id, achievement_id)
SELECT user_id, 'games_100' FROM user_stats WHERE games_played >= 100
ON CONFLICT (user_id, achievement_id) DO NOTHING;

-- First win: any registered player with at least one recorded win.
INSERT INTO user_achievements (user_id, achievement_id)
SELECT user_id, 'first_win' FROM user_stats WHERE wins >= 1
ON CONFLICT (user_id, achievement_id) DO NOTHING;

-- Perfect round: any player who ever finished a game with zero penalty.
INSERT INTO user_achievements (user_id, achievement_id)
SELECT DISTINCT gp.user_id, 'perfect_round'
FROM game_players gp
WHERE gp.user_id IS NOT NULL AND gp.penalty_points = 0
ON CONFLICT (user_id, achievement_id) DO NOTHING;

-- Shared win: any player who shared a win — i.e. a game where two or more
-- registered players were flagged is_winner — and was themselves a winner.
INSERT INTO user_achievements (user_id, achievement_id)
SELECT gp.user_id, 'shared_win'
FROM game_players gp
WHERE gp.user_id IS NOT NULL AND gp.is_winner
  AND gp.game_id IN (
    SELECT game_id FROM game_players
    WHERE user_id IS NOT NULL AND is_winner
    GROUP BY game_id
    HAVING COUNT(*) > 1
  )
ON CONFLICT (user_id, achievement_id) DO NOTHING;
