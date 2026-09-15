import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import type { LeaderboardEntryDto } from '../api/stats'
import { LeaderboardPage } from './LeaderboardPage'

vi.mock('../hooks/useAuth', () => ({
  useAuth: () => ({ token: null }),
}))
vi.mock('../hooks/useEquippedSkins', () => ({
  useEquippedSkinsState: () => ({ skins: [], isLoading: true }),
}))
vi.mock('../api/stats', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/stats')>()
  return {
    ...actual,
    getLeaderboard: vi.fn(),
    getSeasons: vi.fn(),
  }
})

const entry: LeaderboardEntryDto = {
  rank: 1,
  user_id: 'alice-id',
  display_name: 'Alice',
  avatar_url: 'https://cdn.example/alice.png',
  games_played: 20,
  wins: 12,
  win_rate: 0.6,
  avg_penalty: 8,
  best_penalty: 0,
  rating: 1200,
  avg_rank: 1.5,
  top2_rate: 0.8,
  first_place_count: 12,
  human_only_games: 20,
  bot_mixed_games: 0,
  xp: 1500,
  level: 5,
}

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

test('keeps leaderboard fallback avatars hidden while equipped skins load', async () => {
  const { getLeaderboard, getSeasons } = await import('../api/stats')
  vi.mocked(getLeaderboard).mockResolvedValue({
    entries: [entry],
    total: 1,
    page: 1,
    min_games: 10,
    sort: 'win_rate',
    season: '',
  })
  vi.mocked(getSeasons).mockResolvedValue({ seasons: [] })

  render(<MemoryRouter><LeaderboardPage /></MemoryRouter>)

  await waitFor(() => expect(screen.getAllByText('Alice').length).toBeGreaterThan(0))
  expect(screen.getAllByRole('status', { name: 'Loading profile picture' })).toHaveLength(2)
  expect(screen.queryByRole('img', { name: 'Alice' })).not.toBeInTheDocument()
})
