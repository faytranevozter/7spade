import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { HeadlineStats, StatCards } from './StatCards'
import { headlineStats, statGroups } from './statGroups'
import type { UserStatsDto } from '../api/stats'

const stats: UserStatsDto = {
  user_id: 'u1',
  display_name: 'Alice',
  avatar_url: null,
  games_played: 20,
  wins: 8,
  win_rate: 0.4,
  avg_penalty: 6.3,
  best_penalty: 0,
  worst_penalty: 22,
  rating: 1180,
  rank: 4,
  qualified: true,
  avg_rank: 2.15,
  first_place_count: 8,
  second_place_count: 5,
  third_place_count: 4,
  fourth_place_count: 3,
  zero_penalty_games: 2,
  low_penalty_games: 9,
  high_penalty_games: 3,
  human_only_games: 12,
  bot_mixed_games: 8,
  current_win_streak: 1,
  best_win_streak: 4,
  current_top2_streak: 2,
  best_top2_streak: 6,
  close_wins: 3,
  close_losses: 2,
  blowout_wins: 1,
  blowout_losses: 1,
}

afterEach(() => {
  cleanup()
})

test('statGroups exposes all six labelled groups', () => {
  const titles = statGroups(stats).map((g) => g.title)
  expect(titles).toEqual(['Overview', 'Placement', 'Scoring', 'Streaks', 'Clutch', 'Context'])
})

test('headlineStats returns the five summary tiles', () => {
  const tiles = headlineStats(stats)
  expect(tiles.map((t) => t.label)).toEqual(['Rating', 'Rank', 'Games', 'Win rate', 'Avg rank'])
  expect(tiles[0].value).toBe('1180')
  expect(tiles[1].value).toBe('#4')
  expect(tiles[3].value).toBe('40.0%')
})

test('StatCards renders group headings and values', () => {
  render(<StatCards stats={stats} />)
  expect(screen.getByRole('region', { name: 'Overview' })).toBeInTheDocument()
  expect(screen.getByText('Best round')).toBeInTheDocument()
})

test('HeadlineStats renders the summary strip', () => {
  render(<HeadlineStats stats={stats} />)
  const strip = screen.getByLabelText('Headline stats')
  expect(strip).toHaveTextContent('Rating')
  expect(strip).toHaveTextContent('1180')
})

test('rates render as dashes with zero games played', () => {
  const fresh = { ...stats, games_played: 0 }
  expect(headlineStats(fresh).find((t) => t.label === 'Win rate')?.value).toBe('—')
})
