// Achievement catalog — the client-side presentation for each badge. The `id`
// values must stay in sync with the server allowlist in
// services/api/internal/repository/achievements.go (AllAchievementIDs); the
// server only ever awards ids from that list. `icon` is an emoji glyph.
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
]

const achievementById = new Map(achievements.map((a) => [a.id, a]))

export function achievementMeta(id: string): Achievement | undefined {
  return achievementById.get(id)
}
