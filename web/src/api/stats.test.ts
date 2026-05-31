import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from './client'
import { getLeaderboard, getMyStats, getUserStats } from './stats'

describe('stats API', () => {
  beforeEach(() => {
    global.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('fetches the leaderboard with pagination params', async () => {
    const payload = {
      entries: [
        {
          rank: 1,
          user_id: 'u1',
          display_name: 'Alice',
          games_played: 10,
          wins: 7,
          win_rate: 0.7,
          avg_penalty: 12.5,
          best_penalty: 3,
        },
      ],
      total: 1,
      page: 1,
      min_games: 5,
    }
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse(payload))

    const result = await getLeaderboard('tok', 2, 25)

    expect(result).toEqual(payload)
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/leaderboard?page=2&per_page=25',
      expect.objectContaining({
        method: 'GET',
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer tok' },
      }),
    )
  })

  it('fetches the current user stats', async () => {
    const payload = {
      user_id: 'u1',
      display_name: 'Alice',
      games_played: 10,
      wins: 7,
      win_rate: 0.7,
      avg_penalty: 12.5,
      best_penalty: 3,
      rank: 1,
      qualified: true,
    }
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse(payload))

    const result = await getMyStats('tok')

    expect(result).toEqual(payload)
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/stats',
      expect.objectContaining({ headers: { 'Content-Type': 'application/json', Authorization: 'Bearer tok' } }),
    )
  })

  it('fetches a public user profile without a token and encodes the id', async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse({ user_id: 'a b' }))

    await getUserStats(null, 'a b')

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/users/a%20b/stats',
      expect.objectContaining({ headers: { 'Content-Type': 'application/json' } }),
    )
    // No Authorization header when token is null.
    const call = vi.mocked(global.fetch).mock.calls[0][1] as RequestInit
    expect((call.headers as Record<string, string>).Authorization).toBeUndefined()
  })

  it('throws ApiError with the status code on failure', async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse({ error: 'Player not found' }, 404))

    await expect(getUserStats('tok', 'missing')).rejects.toThrow(ApiError)
  })
})

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}
