import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { Text, TextInput, View } from 'react-native'
import { useRouter } from 'expo-router'
import { ApiError } from '../api/client'
import {
  acceptFriendRequest,
  getFriends,
  removeFriend,
  sendFriendRequest,
  type FriendDto,
} from '../api/friends'
import { Avatar } from './Avatar'
import { Button } from './Button'
import { Modal } from './Modal'
import { SectionPanel } from './SectionPanel'
import { initialsForName } from '../game/cards'

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
  const [username, setUsername] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const submit = async () => {
    const handle = username.trim().toLowerCase()
    if (!handle) return
    setBusy(true)
    setError(null)
    setNotice(null)
    try {
      const res = await sendFriendRequest(token, { username: handle })
      if (res.status === 'accepted') {
        onAdded()
      } else {
        setNotice('Request sent')
        onAdded()
      }
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to send request'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title="Add a friend"
      eyebrow="By username"
      description="Enter a player's username to send a friend request."
      onClose={onClose}
      footer={
        <>
          <Button variant="secondary" onPress={onClose}>Cancel</Button>
          <Button onPress={() => void submit()} disabled={busy || username.trim() === ''}>
            {busy ? 'Sending…' : 'Send request'}
          </Button>
        </>
      }
    >
      <View className="gap-2">
        <Text className="text-xs font-medium uppercase text-spade-gray-2">Username</Text>
        <TextInput
          value={username}
          onChangeText={(v) => setUsername(v.toLowerCase())}
          autoCapitalize="none"
          autoCorrect={false}
          maxLength={32}
          className="rounded-spade-md border border-spade-cream/15 bg-spade-bg/70 px-3 py-2 text-sm text-spade-cream"
        />
      </View>
      {error ? <Text className="mt-2 text-xs text-spade-red">{error}</Text> : null}
      {notice ? <Text className="mt-2 text-xs text-spade-gold-light">{notice}</Text> : null}
    </Modal>
  )
}
