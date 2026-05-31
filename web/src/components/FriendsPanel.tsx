import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router'
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

// FriendsPanel renders the caller's accepted friends (with online/in-game
// presence) plus incoming/outgoing requests, and an "add friend by name" flow.
// Hidden for guests by the caller. refreshNonce lets the parent re-poll in sync
// with its other lobby data.
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
          <p className="text-sm text-spade-gray-2">No friends yet — add someone by their display name.</p>
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
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const submit = async () => {
    const displayName = name.trim()
    if (!displayName) return
    setBusy(true)
    setError(null)
    setNotice(null)
    try {
      const res = await sendFriendRequest(token, { displayName })
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
      eyebrow="By display name"
      description="Enter a player's exact display name to send a friend request."
      onClose={onClose}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button onClick={() => void submit()} disabled={busy || name.trim() === ''}>
            {busy ? 'Sending…' : 'Send request'}
          </Button>
        </>
      }
    >
      <label className="grid gap-2">
        <span className="text-xs font-medium uppercase text-spade-gray-2">Display name</span>
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          aria-label="Friend display name"
          className="rounded-spade-md border border-spade-cream/15 bg-spade-bg/70 px-3 py-2 text-sm text-spade-cream outline-none focus:border-spade-gold/50"
        />
      </label>
      {error ? <p role="alert" className="mt-2 text-xs text-spade-red">{error}</p> : null}
      {notice ? <p className="mt-2 text-xs text-spade-gold-light">{notice}</p> : null}
    </Modal>
  )
}
