import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { Text, TextInput, View } from 'react-native'
import { useRouter } from 'expo-router'
import { ApiError } from '../api/client'
import {
  acceptFriendRequest,
  getFriends,
  removeFriend,
  searchUsers,
  sendFriendRequest,
  type FriendDto,
  type UserSearchResultDto,
} from '../api/friends'
import { Avatar } from './Avatar'
import { Button } from './Button'
import { Modal } from './Modal'
import { SectionPanel } from './SectionPanel'
import { initialsForName } from '../game/cards'
import { useDebounce } from '../hooks/useDebounce'

// Minimum characters before a search fires (mirrors the server-side minimum).
const SEARCH_MIN_CHARS = 2

function getErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return fallback
}

// Native port of web/src/components/FriendsPanel.tsx. Accepted friends with
// presence, incoming/outgoing requests, and an add-by-username flow. Hidden for
// guests by the caller.
export function FriendsPanel({ token, refreshNonce }: { token: string | null; refreshNonce: number }) {
  const router = useRouter()
  const [friends, setFriends] = useState<FriendDto[]>([])
  const [error, setError] = useState<string | null>(null)
  const [showAdd, setShowAdd] = useState(false)

  const load = useCallback(() => {
    let cancelled = false
    getFriends(token)
      .then((data) => {
        if (!cancelled) setFriends(data.friends)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [token])

  useEffect(() => load(), [load, refreshNonce])

  const accepted = friends.filter((f) => f.status === 'accepted')
  const incoming = friends.filter((f) => f.status === 'incoming')
  const outgoing = friends.filter((f) => f.status === 'outgoing')

  const act = async (fn: () => Promise<unknown>, fallback: string) => {
    setError(null)
    try {
      await fn()
      load()
    } catch (err) {
      setError(getErrorMessage(err, fallback))
    }
  }

  return (
    <SectionPanel
      title="Friends"
      eyebrow="Your players"
      action={<Button variant="secondary" onPress={() => setShowAdd(true)}>Add friend</Button>}
    >
      {error ? <Text className="mb-3 text-xs text-spade-red">{error}</Text> : null}

      {incoming.length > 0 ? (
        <View className="mb-4 gap-2">
          <Text className="font-mono text-[11px] uppercase tracking-wider text-spade-gold">Requests</Text>
          {incoming.map((f) => (
            <FriendRow key={f.user_id} friend={f}>
              <Button variant="secondary" onPress={() => void act(() => acceptFriendRequest(token, f.user_id), 'Failed to accept')}>Accept</Button>
              <Button variant="ghost" onPress={() => void act(() => removeFriend(token, f.user_id), 'Failed to decline')}>Decline</Button>
            </FriendRow>
          ))}
        </View>
      ) : null}

      <View className="gap-2">
        {accepted.length === 0 && incoming.length === 0 && outgoing.length === 0 ? (
          <Text className="text-sm text-spade-gray-2">No friends yet — add someone by their username.</Text>
        ) : null}
        {accepted.map((f) => (
          <FriendRow key={f.user_id} friend={f}>
            {f.online && f.room_id ? (
              <Button variant="secondary" onPress={() => router.push(`/(app)/spectate/${f.room_id}`)}>Watch</Button>
            ) : null}
            <Button variant="ghost" onPress={() => void act(() => removeFriend(token, f.user_id), 'Failed to remove')}>Remove</Button>
          </FriendRow>
        ))}
        {outgoing.map((f) => (
          <FriendRow key={f.user_id} friend={f}>
            <Text className="font-mono text-[11px] text-spade-gray-3">Pending</Text>
            <Button variant="ghost" onPress={() => void act(() => removeFriend(token, f.user_id), 'Failed to cancel')}>Cancel</Button>
          </FriendRow>
        ))}
      </View>

      {showAdd ? (
        <AddFriendModal
          token={token}
          onClose={() => setShowAdd(false)}
          onAdded={() => {
            setShowAdd(false)
            load()
          }}
        />
      ) : null}
    </SectionPanel>
  )
}

function FriendRow({ friend, children }: { friend: FriendDto; children: ReactNode }) {
  return (
    <View className="flex-row items-center justify-between gap-3 rounded-spade-md border border-spade-cream/10 bg-spade-bg/55 px-3 py-2">
      <View className="flex-1 flex-row items-center gap-2">
        <Avatar avatarUrl={friend.avatar_url} initials={initialsForName(friend.display_name)} size={32} />
        <View className="flex-1">
          <Text className="text-sm font-medium text-spade-cream" numberOfLines={1}>{friend.display_name}</Text>
          {friend.username ? <Text className="font-mono text-[10px] text-spade-gray-2">@{friend.username}</Text> : null}
          {friend.status === 'accepted' ? (
            <Text className="font-mono text-[10px] text-spade-gray-3">
              {friend.online ? (friend.room_id ? 'In a game' : 'Online') : 'Offline'}
            </Text>
          ) : null}
        </View>
      </View>
      <View className="flex-row items-center gap-2">{children}</View>
    </View>
  )
}

function AddFriendModal({ token, onClose, onAdded }: { token: string | null; onClose: () => void; onAdded: () => void }) {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<UserSearchResultDto[]>([])
  const [searching, setSearching] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // Tracks per-row send state by user id: 'busy' while sending, 'sent' after.
  const [sent, setSent] = useState<Record<string, 'busy' | 'sent'>>({})

  const debouncedQuery = useDebounce(query.trim(), 300)
  const tooShort = debouncedQuery.length < SEARCH_MIN_CHARS

  useEffect(() => {
    let cancelled = false
    Promise.resolve()
      .then(() => {
        if (cancelled) return null
        if (tooShort) {
          setResults([])
          setSearching(false)
          return null
        }
        setSearching(true)
        setError(null)
        return searchUsers(token, debouncedQuery)
      })
      .then((res) => {
        if (cancelled || res === null) return
        setResults(res.results)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setError(getErrorMessage(err, 'Search failed'))
      })
      .finally(() => {
        if (cancelled) return
        setSearching(false)
      })
    return () => {
      cancelled = true
    }
  }, [debouncedQuery, tooShort, token])

  const add = async (user: UserSearchResultDto) => {
    setSent((s) => ({ ...s, [user.user_id]: 'busy' }))
    setError(null)
    try {
      await sendFriendRequest(token, { userId: user.user_id })
      setSent((s) => ({ ...s, [user.user_id]: 'sent' }))
      onAdded()
    } catch (err) {
      setSent((s) => {
        const next = { ...s }
        delete next[user.user_id]
        return next
      })
      setError(getErrorMessage(err, 'Failed to send request'))
    }
  }

  return (
    <Modal
      title="Add a friend"
      eyebrow="Find players"
      description="Search by username or display name to send a friend request."
      onClose={onClose}
      footer={<Button variant="secondary" onPress={onClose}>Done</Button>}
    >
      <View className="gap-2">
        <Text className="text-xs font-medium uppercase text-spade-gray-2">Search</Text>
        <TextInput
          value={query}
          onChangeText={setQuery}
          placeholder="username or name"
          autoCapitalize="none"
          autoCorrect={false}
          autoFocus
          maxLength={32}
          className="rounded-spade-md border border-spade-cream/15 bg-spade-bg/70 px-3 py-2 text-sm text-spade-cream"
        />
      </View>

      {error ? <Text className="mt-2 text-xs text-spade-red">{error}</Text> : null}

      <View className="mt-3 gap-2">
        {tooShort ? (
          <Text className="text-xs text-spade-gray-3">Type at least {SEARCH_MIN_CHARS} characters to search.</Text>
        ) : searching ? (
          <Text className="text-xs text-spade-gray-3">Searching…</Text>
        ) : results.length === 0 ? (
          <Text className="text-xs text-spade-gray-2">No players found.</Text>
        ) : (
          results.map((user) => (
            <View
              key={user.user_id}
              className="flex-row items-center justify-between gap-3 rounded-spade-md border border-spade-cream/10 bg-spade-bg/55 px-3 py-2"
            >
              <View className="flex-1 flex-row items-center gap-2">
                <Avatar avatarUrl={user.avatar_url} initials={initialsForName(user.display_name)} size={32} />
                <View className="flex-1">
                  <Text className="text-sm font-medium text-spade-cream" numberOfLines={1}>{user.display_name}</Text>
                  <Text className="font-mono text-[10px] text-spade-gray-2">@{user.username}</Text>
                </View>
              </View>
              {sent[user.user_id] === 'sent' ? (
                <Text className="font-mono text-[11px] text-spade-gold-light">Sent</Text>
              ) : (
                <Button variant="secondary" disabled={sent[user.user_id] === 'busy'} onPress={() => void add(user)}>
                  {sent[user.user_id] === 'busy' ? 'Adding…' : 'Add friend'}
                </Button>
              )}
            </View>
          ))
        )}
      </View>
    </Modal>
  )
}
