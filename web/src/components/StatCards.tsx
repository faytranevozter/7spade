import type { UserStatsDto } from '../api/stats'

type StatCardsProps = {
  stats: UserStatsDto
}

type Tile = { label: string; value: string }
type Group = { title: string; tiles: Tile[] }

// StatCards renders a registered player's lifetime stats as labelled groups of
// read-only tiles, shared by the profile page and the "my stats" panel on the
// history page. Derived rates guard against a zero games_played divisor.
export function StatCards({ stats }: StatCardsProps) {
  const played = stats.games_played
  const top2 = stats.first_place_count + stats.second_place_count
  const groups: Group[] = [
    {
      title: 'Overview',
      tiles: [
        { label: 'Rating', value: String(stats.rating) },
        { label: 'Games played', value: String(played) },
        { label: 'Wins', value: String(stats.wins) },
        { label: 'Win rate', value: played > 0 ? formatPercent(stats.win_rate) : '—' },
        { label: 'Rank', value: stats.qualified && stats.rank !== null ? `#${stats.rank}` : '—' },
      ],
    },
    {
      title: 'Placement',
      tiles: [
        { label: 'Avg rank', value: played > 0 ? stats.avg_rank.toFixed(2) : '—' },
        { label: 'Top 2 rate', value: played > 0 ? formatPercent(top2 / played) : '—' },
        { label: '1st', value: String(stats.first_place_count) },
        { label: '2nd', value: String(stats.second_place_count) },
        { label: '3rd', value: String(stats.third_place_count) },
        { label: '4th', value: String(stats.fourth_place_count) },
      ],
    },
    {
      title: 'Scoring',
      tiles: [
        { label: 'Avg penalty', value: played > 0 ? stats.avg_penalty.toFixed(1) : '—' },
        { label: 'Best round', value: stats.best_penalty === null ? '—' : String(stats.best_penalty) },
        { label: 'Worst round', value: stats.worst_penalty === null ? '—' : String(stats.worst_penalty) },
        { label: 'Zero penalty', value: String(stats.zero_penalty_games) },
        { label: 'Low (≤5)', value: String(stats.low_penalty_games) },
        { label: 'High (≥20)', value: String(stats.high_penalty_games) },
      ],
    },
    {
      title: 'Streaks',
      tiles: [
        { label: 'Win streak', value: String(stats.current_win_streak) },
        { label: 'Best win streak', value: String(stats.best_win_streak) },
        { label: 'Top 2 streak', value: String(stats.current_top2_streak) },
        { label: 'Best top 2', value: String(stats.best_top2_streak) },
      ],
    },
    {
      title: 'Clutch',
      tiles: [
        { label: 'Close wins', value: String(stats.close_wins) },
        { label: 'Close losses', value: String(stats.close_losses) },
        { label: 'Blowout wins', value: String(stats.blowout_wins) },
        { label: 'Blowout losses', value: String(stats.blowout_losses) },
      ],
    },
    {
      title: 'Context',
      tiles: [
        { label: 'Human-only games', value: String(stats.human_only_games) },
        { label: 'Bot-mixed games', value: String(stats.bot_mixed_games) },
      ],
    },
  ]

  return (
    <div className="grid gap-4">
      {groups.map((group) => (
        <StatGroup key={group.title} group={group} />
      ))}
    </div>
  )
}

function StatGroup({ group }: { group: Group }) {
  return (
    <section aria-label={group.title}>
      <h3 className="mb-2 font-mono text-[11px] uppercase tracking-[0.12em] text-spade-gold">{group.title}</h3>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
        {group.tiles.map((tile) => (
          <div
            key={tile.label}
            className="rounded-spade-lg border border-spade-cream/12 bg-[#2b302d] px-4 py-3"
          >
            <p className="font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">{tile.label}</p>
            <p className="mt-1 text-2xl font-medium text-spade-cream">{tile.value}</p>
          </div>
        ))}
      </div>
    </section>
  )
}

function formatPercent(value: number): string {
  return `${(value * 100).toFixed(1)}%`
}
