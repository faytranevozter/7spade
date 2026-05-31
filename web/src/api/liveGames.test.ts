import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { getLiveGames } from './liveGames'

describe('liveGames API', () => {
  beforeEach(() => {
    global.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('fetches in-progress live games', async () => {
    const payload = {
      games: [
        {
          room_id: 'r1',
          invite_code: 'ABC123',
          started_at: '2026-05-09T10:00:00Z',
          player_count: 2,
          players: [
            { user_id: 'u1', display_name: 'Alice' },
            { user_id: 'u2', display_name: 'Bob' },
          ],
        },
      ],
    }
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse(payload))

    const result = await getLiveGames('tok')

    expect(result).toEqual(payload)
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/live-games',
      expect.objectContaining({
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer tok' },
      }),
    )
  })
})

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}
