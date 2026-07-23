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
    xp: 0,
    level: 1,
    xp_into_level: 0,
    xp_for_next_level: 100,
    xp_to_next_level: 100,
    ...overrides,
  }
}

test('renders duel header and opponent name', () => {
  const mine = makeStats({ rating: 1300 })
  const theirs = makeStats({ rating: 1200, display_name: 'Alice' })
  render(<StatComparison mine={mine} theirs={theirs} opponentName="Alice" />)

  expect(screen.getByText('Head to head')).toBeInTheDocument()
  expect(screen.getByText('VS')).toBeInTheDocument()
  expect(screen.getAllByText('You').length).toBeGreaterThan(0)
  expect(screen.getAllByText('Alice').length).toBeGreaterThan(0)
  expect(screen.getByText('You lead')).toBeInTheDocument()
})

test('shows rating delta from the viewer perspective', () => {
  const mine = makeStats({ rating: 1300 })
  const theirs = makeStats({ rating: 1200 })
  render(<StatComparison mine={mine} theirs={theirs} opponentName="Alice" />)
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
  expect(screen.queryByText(/pp$/)).not.toBeInTheDocument()
  expect(screen.getAllByText('—').length).toBeGreaterThan(0)
})

function barWidthsForLabel(label: string): { mine: number; theirs: number } {
  const labelEl = screen.getByText(label)
  const row = labelEl.closest('li')
  expect(row).not.toBeNull()
  const fills = row!.querySelectorAll<HTMLElement>('[style*="width"]')
  expect(fills.length).toBe(2)
  return {
    mine: Number.parseFloat(fills[0].style.width),
    theirs: Number.parseFloat(fills[1].style.width),
  }
}

test('lower-is-better metrics invert bar strength so the better side is longer', () => {
  // Avg penalty: lower wins. Viewer 8 vs opponent 12 → viewer bar should fill fully.
  const { rerender } = render(
    <StatComparison
      mine={makeStats({ avg_penalty: 8 })}
      theirs={makeStats({ avg_penalty: 12 })}
      opponentName="Alice"
    />,
  )

  let bars = barWidthsForLabel('Avg penalty')
  expect(bars.mine).toBeGreaterThan(bars.theirs)
  expect(bars.mine).toBe(100)
  expect(bars.theirs).toBe(0)

  // Viewer worse (15 > 12) → opponent bar longer.
  rerender(
    <StatComparison
      mine={makeStats({ avg_penalty: 15 })}
      theirs={makeStats({ avg_penalty: 12 })}
      opponentName="Alice"
    />,
  )
  bars = barWidthsForLabel('Avg penalty')
  expect(bars.theirs).toBeGreaterThan(bars.mine)
  expect(bars.theirs).toBe(100)
  expect(bars.mine).toBe(0)

  // Best round is also lower-is-better.
  rerender(
    <StatComparison
      mine={makeStats({ best_penalty: 0 })}
      theirs={makeStats({ best_penalty: 5 })}
      opponentName="Alice"
    />,
  )
  bars = barWidthsForLabel('Best round')
  expect(bars.mine).toBeGreaterThan(bars.theirs)

  // Contrast: higher-is-better rating does NOT invert (higher raw value → longer bar).
  rerender(
    <StatComparison
      mine={makeStats({ rating: 1300 })}
      theirs={makeStats({ rating: 1000 })}
      opponentName="Alice"
    />,
  )
  bars = barWidthsForLabel('Rating')
  expect(bars.mine).toBeGreaterThan(bars.theirs)
  expect(bars.mine).toBe(100)
})
