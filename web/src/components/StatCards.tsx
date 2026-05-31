import type { UserStatsDto } from '../api/stats'

type StatCardsProps = {
  stats: UserStatsDto
}

// StatCards renders a registered player's lifetime stats as a read-only grid,
// shared by the profile page and the "my stats" panel on the history page.
export function StatCards({ stats }: StatCardsProps) {
  const cards: { label: string; value: string }[] = [
    { label: 'Games played', value: String(stats.games_played) },
    { label: 'Wins', value: String(stats.wins) },
    { label: 'Win rate', value: formatPercent(stats.win_rate) },
    { label: 'Avg penalty', value: stats.games_played > 0 ? stats.avg_penalty.toFixed(1) : '—' },
    { label: 'Best round', value: stats.best_penalty === null ? '—' : String(stats.best_penalty) },
    { label: 'Rank', value: stats.qualified && stats.rank !== null ? `#${stats.rank}` : '—' },
  ]

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
      {cards.map((card) => (
        <div
          key={card.label}
          className="rounded-spade-lg border border-spade-cream/12 bg-[#2b302d] px-4 py-3"
        >
          <p className="font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">{card.label}</p>
          <p className="mt-1 text-2xl font-medium text-spade-cream">{card.value}</p>
        </div>
      ))}
    </div>
  )
}

function formatPercent(value: number): string {
  return `${(value * 100).toFixed(1)}%`
}
