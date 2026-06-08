import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { acceptFriendRequest, getFriends, removeFriend, searchUsers, sendFriendRequest } from './friends'

describe('friends API', () => {
  beforeEach(() => {
    global.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('fetches the friends list', async () => {
    const payload = {
      friends: [
        { user_id: 'u1', display_name: 'Alice', username: 'alice', avatar_url: null, status: 'accepted', online: true, room_id: 'r1' },
      ],
    }
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse(payload))

    const result = await getFriends('tok')

    expect(result).toEqual(payload)
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/friends',
      expect.objectContaining({ headers: { 'Content-Type': 'application/json', Authorization: 'Bearer tok' } }),
    )
  })

  it('sends a friend request by username', async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse({ status: 'pending' }))

    const res = await sendFriendRequest('tok', { username: 'bob' })

    expect(res).toEqual({ status: 'pending' })
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/friends/requests',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ username: 'bob' }),
      }),
    )
  })

  it('accepts a request (204, no body)', async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(new Response(null, { status: 204 }))

    await expect(acceptFriendRequest('tok', 'u2')).resolves.toBeUndefined()
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/friends/requests/u2/accept',
      expect.objectContaining({ method: 'POST' }),
    )
  })

  it('removes a friend (DELETE, encoded id)', async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(new Response(null, { status: 204 }))

    await removeFriend('tok', 'a b')
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/friends/a%20b',
      expect.objectContaining({ method: 'DELETE' }),
    )
  })

  it('searches users with an encoded query', async () => {
    const payload = {
      results: [{ user_id: 'u1', username: 'alice', display_name: 'Alice', avatar_url: null }],
    }
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse(payload))

    const res = await searchUsers('tok', 'al ice')

    expect(res).toEqual(payload)
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/users/search?q=al%20ice',
      expect.objectContaining({ headers: { 'Content-Type': 'application/json', Authorization: 'Bearer tok' } }),
    )
  })
})

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}
