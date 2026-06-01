import { apiRequest } from './client'

export type FriendDto = {
  user_id: string
  display_name: string
  username: string
  avatar_url: string | null
  // 'accepted' | 'incoming' | 'outgoing'
  status: string
  online: boolean
  room_id?: string
}

export type FriendsResponse = {
  friends: FriendDto[]
}

export function getFriends(token: string | null): Promise<FriendsResponse> {
  return apiRequest<FriendsResponse>('/friends', { token })
}

// sendFriendRequest targets a user by exact (lowercase) username or user id.
// Returns { status: 'pending' | 'accepted' }.
export function sendFriendRequest(
  token: string | null,
  target: { username?: string; userId?: string },
): Promise<{ status: string }> {
  const body: Record<string, string> = {}
  if (target.userId) body.user_id = target.userId
  if (target.username) body.username = target.username
  return apiRequest<{ status: string }>('/friends/requests', { method: 'POST', token, body })
}

export function acceptFriendRequest(token: string | null, userId: string): Promise<void> {
  return apiRequest<void>(`/friends/requests/${encodeURIComponent(userId)}/accept`, { method: 'POST', token })
}

// removeFriend declines, cancels, or unfriends (single endpoint, both directions).
export function removeFriend(token: string | null, userId: string): Promise<void> {
  return apiRequest<void>(`/friends/${encodeURIComponent(userId)}`, { method: 'DELETE', token })
}
