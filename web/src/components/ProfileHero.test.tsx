import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import type { UserStatsDto } from '../api/stats'
import { ProfileHero } from './ProfileHero'

const baseStats: UserStatsDto = {
  user_id: 'u1',
  display_name: 'Alice',
  avatar_url: null,
  games_played: 10,
  wins: 4,
  win_rate: 0.4,
  avg_penalty: 12,
  best_penalty: 0,
  worst_penalty: 40,
  rating: 1200,
  rank: 3,
  qualified: true,
  avg_rank: 2.1,
  first_place_count: 4,
  second_place_count: 2,
  third_place_count: 2,
  fourth_place_count: 2,
  zero_penalty_games: 1,
  low_penalty_games: 3,
  high_penalty_games: 1,
  human_only_games: 8,
  bot_mixed_games: 2,
  current_win_streak: 1,
  best_win_streak: 3,
  current_top2_streak: 1,
  best_top2_streak: 4,
  close_wins: 1,
  close_losses: 1,
  blowout_wins: 1,
  blowout_losses: 0,
  xp: 1500,
  level: 5,
  xp_into_level: 100,
  xp_for_next_level: 400,
  xp_to_next_level: 300,
}

afterEach(() => {
  cleanup()
})

test('renders name, handle, level, and headline stats', () => {
  render(
    <ProfileHero
      displayName="Alice"
      username="alice"
      stats={baseStats}
      actions={<button type="button">Edit name</button>}
    />,
  )

  expect(screen.getByText('Alice')).toBeInTheDocument()
  expect(screen.getByText('@alice')).toBeInTheDocument()
  expect(screen.getByText('Lv 5')).toBeInTheDocument()
  expect(screen.getByRole('progressbar')).toBeInTheDocument()
  expect(screen.getByLabelText('Headline stats')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Edit name' })).toBeInTheDocument()
})

test('guest-style hero shows meta without level bar', () => {
  render(<ProfileHero displayName="Guest" stats={null} meta="Guest player" />)

  expect(screen.getByText('Guest')).toBeInTheDocument()
  expect(screen.getByText('Guest player')).toBeInTheDocument()
  expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('Headline stats')).not.toBeInTheDocument()
})
