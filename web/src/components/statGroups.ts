import type { UserStatsDto } from '../api/stats'

export type StatTile = { label: string; value: string }
export type StatGroup = { title: string; tiles: StatTile[] }

function formatPercent(value: number): string {
  return `${(value * 100).toFixed(1)}%`
}

// statGroups builds the labelled stat groups for a player, shared by the full
// card list and the headline strip. Derived rates guard against a zero
// games_played divisor.
export function statGroups(stats: UserStatsDto): StatGroup[] {
  const played = stats.games_played
  const top2 = stats.first_place_count + stats.second_place_count
  return [
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
}

// headlineStats is the compact always-visible summary shown above the tabs:
// the five numbers a viewer cares about most at a glance.
export function headlineStats(stats: UserStatsDto): StatTile[] {
  const played = stats.games_played
  return [
    { label: 'Rating', value: String(stats.rating) },
    { label: 'Rank', value: stats.qualified && stats.rank !== null ? `#${stats.rank}` : '—' },
    { label: 'Games', value: String(played) },
    { label: 'Win rate', value: played > 0 ? formatPercent(stats.win_rate) : '—' },
    { label: 'Avg rank', value: played > 0 ? stats.avg_rank.toFixed(2) : '—' },
  ]
}
