import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from './client'
import { getLeaderboard, getMyStats, getSeasons, getUserStats } from './stats'

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
      sort: 'total_wins',
    }
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse(payload))

    const result = await getLeaderboard('tok', 2, 25, 'total_wins')

    expect(result).toEqual(payload)
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/leaderboard?page=2&per_page=25&sort=total_wins',
      expect.objectContaining({
        method: 'GET',
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer tok' },
      }),
    )
  })

  it('defaults the leaderboard sort to win_rate', async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      jsonResponse({ entries: [], total: 0, page: 1, min_games: 5, sort: 'win_rate' }),
    )

    await getLeaderboard('tok', 1, 10)

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/leaderboard?page=1&per_page=10&sort=win_rate',
      expect.objectContaining({ method: 'GET' }),
    )
  })

  it('passes the season scope and rating sort to the leaderboard', async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      jsonResponse({ entries: [], total: 0, page: 1, min_games: 5, sort: 'rating', season: '2026-06' }),
    )

    await getLeaderboard('tok', 1, 10, 'rating', 'active')

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/leaderboard?page=1&per_page=10&sort=rating&season=active',
      expect.objectContaining({ method: 'GET' }),
    )
  })

  it('omits the season param for the all-time scope', async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      jsonResponse({ entries: [], total: 0, page: 1, min_games: 5, sort: 'win_rate', season: '' }),
    )

    await getLeaderboard('tok', 1, 10, 'win_rate', '')

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/leaderboard?page=1&per_page=10&sort=win_rate',
      expect.objectContaining({ method: 'GET' }),
    )
  })

  it('lists seasons', async () => {
    const payload = {
      seasons: [{ id: '2026-06', label: 'June 2026', started_at: '2026-06-01T00:00:00Z', ended_at: null, active: true }],
    }
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse(payload))

    const result = await getSeasons('tok')

    expect(result).toEqual(payload)
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/seasons',
      expect.objectContaining({ method: 'GET' }),
    )
  })

  it('scopes personal stats to a season', async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse({ user_id: 'u1' }))

    await getMyStats('tok', '2026-06')

    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/stats?season=2026-06',
      expect.objectContaining({ method: 'GET' }),
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
