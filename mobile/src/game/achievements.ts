// Achievement catalog — ported verbatim from web/src/game/achievements.ts. The
// `id` values must stay in sync with the server allowlist in
// services/api/internal/repository/achievements.go (AllAchievementIDs).
export type Achievement = {
  id: string
  name: string
  description: string
  icon: string
}

export const achievements: Achievement[] = [
  { id: 'first_win', name: 'First Blood', description: 'Win your first game', icon: '🏆' },
  { id: 'games_10', name: 'Regular', description: 'Play 10 games', icon: '🎴' },
  { id: 'games_50', name: 'Veteran', description: 'Play 50 games', icon: '🎖️' },
  { id: 'games_100', name: 'Centurion', description: 'Play 100 games', icon: '💯' },
  { id: 'streak_3', name: 'On a Roll', description: 'Win 3 games in a row', icon: '🔥' },
  { id: 'streak_5', name: 'Unstoppable', description: 'Win 5 games in a row', icon: '⚡' },
  { id: 'perfect_round', name: 'Flawless', description: 'Finish a game with zero penalty', icon: '✨' },
  { id: 'shared_win', name: 'Good Company', description: 'Share a win in a tie', icon: '🤝' },
  { id: 'wins_50', name: 'Champion', description: 'Win 50 games', icon: '🥇' },
  { id: 'wins_100', name: 'Legend', description: 'Win 100 games', icon: '👑' },
  { id: 'streak_10', name: 'Legendary', description: 'Win 10 games in a row', icon: '🌪️' },
  { id: 'streak_15', name: 'Mythical', description: 'Win 15 games in a row', icon: '🐉' },
  { id: 'firsts_50', name: 'Sovereign', description: 'Finish 1st in 50 games', icon: '💎' },
  { id: 'firsts_100', name: 'Emperor', description: 'Finish 1st in 100 games', icon: '🏆' },
  { id: 'zero_penalty_games_10', name: 'Zen Master', description: 'Finish 10 games with zero penalty', icon: '🧘' },
  { id: 'games_200', name: 'Lifetimer', description: 'Play 200 games', icon: '🗻' },
  { id: 'human_only_25', name: 'Social Butterfly', description: 'Play 25 human-only games', icon: '🦋' },
]

const achievementById = new Map(achievements.map((a) => [a.id, a]))

export function achievementMeta(id: string): Achievement | undefined {
  return achievementById.get(id)
}
