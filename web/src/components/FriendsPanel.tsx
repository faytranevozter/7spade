import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router'
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

// FriendsPanel renders the caller's accepted friends (with online/in-game
// presence) plus incoming/outgoing requests, and an "add friend by username"
// flow. Hidden for guests by the caller. refreshNonce lets the parent re-poll in
// sync with its other lobby data.
export function FriendsPanel({ token, refreshNonce }: { token: string | null; refreshNonce: number }) {
  const navigate = useNavigate()
  const [friends, setFriends] = useState<FriendDto[]>([])
  const [error, setError] = useState<string | null>(null)
  const [showAdd, setShowAdd] = useState(false)

  const load = useCallback(() => {
    let cancelled = false
    getFriends(token)
      .then((data) => {
        if (!cancelled) setFriends(data.friends)
      })
      .catch(() => {
        // Non-fatal; leave the list as-is.
      })
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

  const action = (
    <Button variant="secondary" onClick={() => setShowAdd(true)}>
      Add friend
    </Button>
  )

  return (
    <SectionPanel title="Friends" eyebrow="Your players" action={action}>
      {error ? (
        <p role="alert" className="mb-3 text-xs text-spade-red">
          {error}
        </p>
      ) : null}

      {incoming.length > 0 ? (
        <div className="mb-4 grid gap-2">
          <h4 className="font-mono text-[11px] uppercase tracking-[0.12em] text-spade-gold">Requests</h4>
          {incoming.map((f) => (
            <FriendRow key={f.user_id} friend={f}>
              <Button variant="secondary" onClick={() => void act(() => acceptFriendRequest(token, f.user_id), 'Failed to accept')}>
                Accept
              </Button>
              <Button variant="ghost" onClick={() => void act(() => removeFriend(token, f.user_id), 'Failed to decline')}>
                Decline
              </Button>
            </FriendRow>
          ))}
        </div>
      ) : null}

      <div className="grid gap-2">
        {accepted.length === 0 && incoming.length === 0 && outgoing.length === 0 ? (
          <p className="text-sm text-spade-gray-2">No friends yet — add someone by their username.</p>
        ) : null}
        {accepted.map((f) => (
          <FriendRow key={f.user_id} friend={f}>
            {f.online && f.room_id ? (
              <Button variant="secondary" onClick={() => navigate(`/watch/${f.room_id}`)}>
                Watch
              </Button>
            ) : null}
            <Button variant="ghost" onClick={() => void act(() => removeFriend(token, f.user_id), 'Failed to remove')}>
              Remove
            </Button>
          </FriendRow>
        ))}
        {outgoing.map((f) => (
          <FriendRow key={f.user_id} friend={f}>
            <span className="font-mono text-[11px] text-spade-gray-3">Pending</span>
            <Button variant="ghost" onClick={() => void act(() => removeFriend(token, f.user_id), 'Failed to cancel')}>
              Cancel
            </Button>
          </FriendRow>
        ))}
      </div>

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
    <div className="flex items-center justify-between gap-3 rounded-spade-md border border-spade-cream/10 bg-spade-bg/55 px-3 py-2">
      <div className="flex min-w-0 items-center gap-2">
        <span className="relative">
          <Avatar avatarUrl={friend.avatar_url} initials={initialsForName(friend.display_name)} sizeClass="size-8" className="text-xs" />
          {friend.status === 'accepted' ? (
            <span
              aria-label={friend.online ? 'online' : 'offline'}
              className={`absolute -bottom-0.5 -right-0.5 size-2.5 rounded-full border border-spade-bg ${friend.online ? 'bg-green-400' : 'bg-spade-gray-3'}`}
            />
          ) : null}
        </span>
        <div className="min-w-0">
          <p className="truncate text-sm font-medium text-spade-cream">{friend.display_name}</p>
          {friend.username ? (
            <p className="truncate font-mono text-[10px] text-spade-gray-2">@{friend.username}</p>
          ) : null}
          {friend.status === 'accepted' ? (
            <p className="font-mono text-[10px] text-spade-gray-3">
              {friend.online ? (friend.room_id ? 'In a game' : 'Online') : 'Offline'}
            </p>
          ) : null}
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-2">{children}</div>
    </div>
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
      footer={
        <Button variant="secondary" onClick={onClose}>
          Done
        </Button>
      }
    >
      <label className="grid gap-2">
        <span className="text-xs font-medium uppercase text-spade-gray-2">Search</span>
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          aria-label="Search players"
          placeholder="username or name"
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
          maxLength={32}
          autoFocus
          className="rounded-spade-md border border-spade-cream/15 bg-spade-bg/70 px-3 py-2 text-sm text-spade-cream outline-none focus:border-spade-gold/50"
        />
      </label>

      {error ? <p role="alert" className="mt-2 text-xs text-spade-red">{error}</p> : null}

      <div className="mt-3 grid gap-2">
        {tooShort ? (
          <p className="text-xs text-spade-gray-3">Type at least {SEARCH_MIN_CHARS} characters to search.</p>
        ) : searching ? (
          <p className="text-xs text-spade-gray-3">Searching…</p>
        ) : results.length === 0 ? (
          <p className="text-xs text-spade-gray-2">No players found.</p>
        ) : (
          results.map((user) => (
            <div
              key={user.user_id}
              className="flex items-center justify-between gap-3 rounded-spade-md border border-spade-cream/10 bg-spade-bg/55 px-3 py-2"
            >
              <div className="flex min-w-0 items-center gap-2">
                <Avatar avatarUrl={user.avatar_url} initials={initialsForName(user.display_name)} sizeClass="size-8" className="text-xs" />
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium text-spade-cream">{user.display_name}</p>
                  <p className="truncate font-mono text-[10px] text-spade-gray-2">@{user.username}</p>
                </div>
              </div>
              {sent[user.user_id] === 'sent' ? (
                <span className="font-mono text-[11px] text-spade-gold-light">Sent</span>
              ) : (
                <Button
                  variant="secondary"
                  disabled={sent[user.user_id] === 'busy'}
                  onClick={() => void add(user)}
                >
                  {sent[user.user_id] === 'busy' ? 'Adding…' : 'Add friend'}
                </Button>
              )}
            </div>
          ))
        )}
      </div>
    </Modal>
  )
}
