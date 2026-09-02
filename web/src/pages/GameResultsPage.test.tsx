import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import { getGameResults } from '../api/history'
import { useAuth } from '../hooks/useAuth'
import { GameResultsPage } from './GameResultsPage'

vi.mock('../api/history', () => ({ getGameResults: vi.fn() }))
vi.mock('../hooks/useAuth', () => ({
  useAuth: vi.fn(() => ({ token: 'test-token', isAuthenticated: true })),
}))

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

test('shows XP but no rating for a saved game without a registered opponent', async () => {
  vi.mocked(getGameResults).mockResolvedValue({
    game_id: 'game-1',
    room_id: 'room-1',
    room_name: 'Guest table',
    started_at: '2026-01-01T10:00:00Z',
    finished_at: '2026-01-01T10:05:00Z',
    replay_available: false,
    players: [
      {
        player_index: 0,
        user_id: 'user-1',
        display_name: 'Alice',
        penalty_points: 3,
        rank: 1,
        is_winner: true,
        is_bot: false,
        is_guest: false,
        is_me: true,
        facedown_cards: [],
        xp_delta: 125,
        xp_after: 125,
        level: 2,
      },
      {
        player_index: 1,
        user_id: 'guest-1',
        display_name: 'Guest',
        penalty_points: 7,
        rank: 2,
        is_winner: false,
        is_bot: false,
        is_guest: true,
        is_me: false,
        facedown_cards: [],
      },
    ],
  })

  render(
    <MemoryRouter initialEntries={['/games/game-1/results']}>
      <Routes>
        <Route path="/games/:gameId/results" element={<GameResultsPage />} />
      </Routes>
    </MemoryRouter>,
  )

  expect(await screen.findByText('XP gained')).toBeInTheDocument()
  expect(screen.getByText('+125')).toBeInTheDocument()
  expect(screen.queryByText('Rating')).not.toBeInTheDocument()
  expect(useAuth).toHaveBeenCalled()
})
