import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { StatComparison } from './StatComparison'
import type { UserStatsDto } from '../api/stats'

afterEach(cleanup)

function makeStats(overrides: Partial<UserStatsDto>): UserStatsDto {
  return {
    user_id: 'u',
    display_name: 'Player',
    avatar_url: null,
    games_played: 10,
    wins: 5,
    win_rate: 0.5,
    avg_penalty: 12,
    best_penalty: 3,
    worst_penalty: 24,
    rating: 1200,
    rank: null,
    qualified: false,
    avg_rank: 2.5,
    first_place_count: 3,
    second_place_count: 2,
    third_place_count: 3,
    fourth_place_count: 2,
    zero_penalty_games: 1,
    low_penalty_games: 4,
    high_penalty_games: 2,
    human_only_games: 6,
    bot_mixed_games: 4,
    current_win_streak: 1,
    best_win_streak: 3,
    current_top2_streak: 2,
    best_top2_streak: 4,
    close_wins: 2,
    close_losses: 1,
    blowout_wins: 1,
    blowout_losses: 1,
    ...overrides,
  }
}

test('renders both columns and the opponent name', () => {
  const mine = makeStats({ rating: 1300 })
  const theirs = makeStats({ rating: 1200, display_name: 'Alice' })
  render(<StatComparison mine={mine} theirs={theirs} opponentName="Alice" />)

  expect(screen.getByText('You vs Alice')).toBeInTheDocument()
  expect(screen.getByText('You')).toBeInTheDocument()
  // Opponent name appears in the header and the section label.
  expect(screen.getAllByText('Alice').length).toBeGreaterThan(0)
})

test('shows rating delta from the viewer perspective', () => {
  const mine = makeStats({ rating: 1300 })
  const theirs = makeStats({ rating: 1200 })
  render(<StatComparison mine={mine} theirs={theirs} opponentName="Alice" />)
  // +100 rating advantage.
  expect(screen.getByText('+100')).toBeInTheDocument()
})

test('win-rate delta is shown in percentage points', () => {
  const mine = makeStats({ win_rate: 0.6, games_played: 10 })
  const theirs = makeStats({ win_rate: 0.5, games_played: 10 })
  render(<StatComparison mine={mine} theirs={theirs} opponentName="Alice" />)
  expect(screen.getByText('+10.0pp')).toBeInTheDocument()
})

test('lower avg penalty reads as an advantage (green)', () => {
  const mine = makeStats({ avg_penalty: 8 })
  const theirs = makeStats({ avg_penalty: 12 })
  render(<StatComparison mine={mine} theirs={theirs} opponentName="Alice" />)
  // mine - theirs = -4 (lower is better here).
  const delta = screen.getByText('-4')
  expect(delta).toHaveClass('text-green-400')
})

test('higher avg penalty reads as worse (red)', () => {
  const mine = makeStats({ avg_penalty: 15 })
  const theirs = makeStats({ avg_penalty: 12 })
  render(<StatComparison mine={mine} theirs={theirs} opponentName="Alice" />)
  const delta = screen.getByText('+3')
  expect(delta).toHaveClass('text-spade-red')
})

test('viewer with zero games shows dashes and no win-rate delta', () => {
  const mine = makeStats({ games_played: 0, win_rate: 0, avg_penalty: 0, best_penalty: null })
  const theirs = makeStats({ games_played: 10, win_rate: 0.5, avg_penalty: 12, best_penalty: 3 })
  render(<StatComparison mine={mine} theirs={theirs} opponentName="Alice" />)
  // No NaN/percentage-point delta when the viewer has no games.
  expect(screen.queryByText(/pp$/)).not.toBeInTheDocument()
  // Both win-rate cells render as dashes for the viewer side.
  expect(screen.getAllByText('—').length).toBeGreaterThan(0)
})
