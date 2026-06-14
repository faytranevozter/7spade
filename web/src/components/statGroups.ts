import type { UserStatsDto } from '../api/stats'

export type StatTile = { label: string; value: string; icon: string }
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
        { label: 'Rating', value: String(stats.rating), icon: '📊' },
        { label: 'Games played', value: String(played), icon: '🎲' },
        { label: 'Wins', value: String(stats.wins), icon: '🏆' },
        { label: 'Win rate', value: played > 0 ? formatPercent(stats.win_rate) : '—', icon: '📈' },
        { label: 'Rank', value: stats.qualified && stats.rank !== null ? `#${stats.rank}` : '—', icon: '🥇' },
      ],
    },
    {
      title: 'Placement',
      tiles: [
        { label: 'Avg rank', value: played > 0 ? stats.avg_rank.toFixed(2) : '—', icon: '🎯' },
        { label: 'Top 2 rate', value: played > 0 ? formatPercent(top2 / played) : '—', icon: '🔝' },
        { label: '1st', value: String(stats.first_place_count), icon: '🥇' },
        { label: '2nd', value: String(stats.second_place_count), icon: '🥈' },
        { label: '3rd', value: String(stats.third_place_count), icon: '🥉' },
        { label: '4th', value: String(stats.fourth_place_count), icon: '🎴' },
      ],
    },
    {
      title: 'Scoring',
      tiles: [
        { label: 'Avg penalty', value: played > 0 ? stats.avg_penalty.toFixed(1) : '—', icon: '➗' },
        { label: 'Best round', value: stats.best_penalty === null ? '—' : String(stats.best_penalty), icon: '🌟' },
        { label: 'Worst round', value: stats.worst_penalty === null ? '—' : String(stats.worst_penalty), icon: '💀' },
        { label: 'Zero penalty', value: String(stats.zero_penalty_games), icon: '✨' },
        { label: 'Low (≤5)', value: String(stats.low_penalty_games), icon: '🍃' },
        { label: 'High (≥20)', value: String(stats.high_penalty_games), icon: '🔥' },
      ],
    },
    {
      title: 'Streaks',
      tiles: [
        { label: 'Win streak', value: String(stats.current_win_streak), icon: '⚡' },
        { label: 'Best win streak', value: String(stats.best_win_streak), icon: '🔥' },
        { label: 'Top 2 streak', value: String(stats.current_top2_streak), icon: '🔗' },
        { label: 'Best top 2', value: String(stats.best_top2_streak), icon: '🏅' },
      ],
    },
    {
      title: 'Clutch',
      tiles: [
        { label: 'Close wins', value: String(stats.close_wins), icon: '😅' },
        { label: 'Close losses', value: String(stats.close_losses), icon: '😬' },
        { label: 'Blowout wins', value: String(stats.blowout_wins), icon: '💥' },
        { label: 'Blowout losses', value: String(stats.blowout_losses), icon: '🧊' },
      ],
    },
    {
      title: 'Context',
      tiles: [
        { label: 'Human-only games', value: String(stats.human_only_games), icon: '🧑' },
        { label: 'Bot-mixed games', value: String(stats.bot_mixed_games), icon: '🤖' },
      ],
    },
  ]
}

// headlineStats is the compact always-visible summary shown above the tabs:
// the five numbers a viewer cares about most at a glance.
export function headlineStats(stats: UserStatsDto): StatTile[] {
  const played = stats.games_played
  return [
    { label: 'Rating', value: String(stats.rating), icon: '📊' },
    { label: 'Rank', value: stats.qualified && stats.rank !== null ? `#${stats.rank}` : '—', icon: '🥇' },
    { label: 'Games', value: String(played), icon: '🎲' },
    { label: 'Win rate', value: played > 0 ? formatPercent(stats.win_rate) : '—', icon: '📈' },
    { label: 'Avg rank', value: played > 0 ? stats.avg_rank.toFixed(2) : '—', icon: '🎯' },
  ]
}
