import { type FormEvent, useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { Modal } from '../components/Modal'
import { RoomCard } from '../components/RoomCard'
import { SceneShell } from '../components/SceneShell'
import { ApiError } from '../api/client'
import {
  getRooms,
  postJoinRoom,
  postRoom,
  type RoomDto,
  type RoomVisibility,
} from '../api/lobby'
import { useAuth } from '../hooks/useAuth'
import type { Room } from '../types'

const TIMER_OPTIONS: ReadonlyArray<30 | 60 | 90 | 120> = [30, 60, 90, 120]

function roomDtoToRoom(dto: RoomDto): Room {
  const fillStatus = dto.player_count >= 4 ? 'Full' : `${dto.player_count} / 4 players`
  return {
    name: dto.visibility === 'private' ? 'Private room' : 'Public room',
    code: dto.invite_code,
    players: `${dto.player_count} / 4`,
    status: dto.status === 'waiting' ? fillStatus : `Status: ${dto.status}`,
    timer: `${dto.turn_timer_seconds}s`,
    open: dto.status === 'waiting' && dto.player_count < 4,
    filledSeats: Math.min(dto.player_count, 4),
    maxSeats: 4,
    visibility: dto.visibility,
  }
}

function getErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return fallback
}

export function LobbyPage() {
  const navigate = useNavigate()
  const { token, isAuthenticated } = useAuth()

  const [rooms, setRooms] = useState<RoomDto[]>([])
  const [isLoadingRooms, setIsLoadingRooms] = useState(false)
  const [listError, setListError] = useState<string | null>(null)

  const [visibility, setVisibility] = useState<RoomVisibility>('public')
  const [timer, setTimer] = useState<30 | 60 | 90 | 120>(60)
  const [isCreating, setIsCreating] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)
  const [showCreate, setShowCreate] = useState(false)

  const [inviteCode, setInviteCode] = useState('')
  const [isJoining, setIsJoining] = useState(false)
  const [joinError, setJoinError] = useState<string | null>(null)
  const [showJoin, setShowJoin] = useState(false)

  const [refreshNonce, setRefreshNonce] = useState(0)

  useEffect(() => {
    if (!isAuthenticated) {
      navigate('/auth', { replace: true })
    }
  }, [isAuthenticated, navigate])

  const loadRooms = useCallback(
    (background: boolean) => {
      if (!isAuthenticated) return () => {}
      let cancelled = false
      Promise.resolve()
        .then(() => {
          if (cancelled) return null
          // Background polls refresh silently so the list doesn't flash a
          // "Refreshing…" state or clear a visible error every few seconds.
          if (!background) {
            setIsLoadingRooms(true)
            setListError(null)
          }
          return getRooms(token)
        })
        .then((data) => {
          if (cancelled || data === null) return
          setRooms(data)
          if (background) setListError(null)
        })
        .catch((err: unknown) => {
          if (cancelled) return
          setListError(getErrorMessage(err, 'Failed to load rooms'))
        })
        .finally(() => {
          if (cancelled || background) return
          setIsLoadingRooms(false)
        })
      return () => {
        cancelled = true
      }
    },
    [isAuthenticated, token],
  )

  // Initial load + explicit refreshes (mount, refresh button, after join/create
  // errors) run with the loading indicator shown.
  useEffect(() => loadRooms(false), [loadRooms, refreshNonce])

  // Auto-refresh the list so rooms that fill up, start, or get deleted (e.g.
  // when their last player leaves) drop off without a manual refresh.
  useEffect(() => {
    if (!isAuthenticated) return
    const interval = window.setInterval(() => loadRooms(true), 5000)
    return () => window.clearInterval(interval)
  }, [isAuthenticated, loadRooms])

  const refreshRooms = useCallback(() => {
    setRefreshNonce((n) => n + 1)
  }, [])

  const handleCreateRoom = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setCreateError(null)
    setIsCreating(true)
    try {
      const created = await postRoom(token, {
        visibility,
        turn_timer_seconds: timer,
      })
      navigate(`/room/${created.id}`)
    } catch (err) {
      setCreateError(getErrorMessage(err, 'Failed to create room'))
    } finally {
      setIsCreating(false)
    }
  }

  const handleJoinByCode = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const code = inviteCode.trim().toUpperCase()
    if (!code) {
      setJoinError('Enter an invite code')
      return
    }
    setJoinError(null)
    setIsJoining(true)
    try {
      const joined = await postJoinRoom(token, code)
      navigate(`/room/${joined.id}`)
    } catch (err) {
      setJoinError(getErrorMessage(err, 'Failed to join room'))
    } finally {
      setIsJoining(false)
    }
  }

  const handleJoinPublic = async (room: RoomDto) => {
    setJoinError(null)
    try {
      const joined = await postJoinRoom(token, room.invite_code)
      navigate(`/room/${joined.id}`)
    } catch (err) {
      setJoinError(getErrorMessage(err, 'Failed to join room'))
    }
  }

  const openCreate = () => {
    setCreateError(null)
    setShowCreate(true)
  }

  const openJoin = () => {
    setJoinError(null)
    setShowJoin(true)
  }

  const openRoomCount = rooms.filter((room) => room.status === 'waiting' && room.player_count < 4).length

  return (
    <SceneShell
      title="Game lobby"
      eyebrow="Lobby"
      action={
        <div className="flex flex-wrap items-center gap-2">
          <Badge tone="waiting">{`${openRoomCount} waiting`}</Badge>
          <Button onClick={openCreate}>Create room</Button>
          <Button variant="secondary" onClick={openJoin}>Join by code</Button>
        </div>
      }
    >
      <div className="grid content-start gap-3">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-medium text-spade-gray-2">Public rooms</h3>
          <button
            type="button"
            onClick={() => refreshRooms()}
            className="font-mono text-xs uppercase tracking-[0.12em] text-spade-gold hover:text-spade-gold-light"
          >
            {isLoadingRooms ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>
        {listError ? (
          <p role="alert" className="text-xs text-spade-red">
            {listError}
          </p>
        ) : null}
        {joinError && !showJoin ? (
          <p role="alert" className="text-xs text-spade-red">
            {joinError}
          </p>
        ) : null}
        {!isLoadingRooms && rooms.length === 0 && !listError ? (
          <div className="rounded-spade-lg border border-dashed border-spade-cream/15 bg-spade-bg/40 p-10 text-center">
            <p className="text-sm text-spade-gray-2">No public rooms waiting.</p>
            <p className="mt-1 text-xs text-spade-gray-3">Create one to get the table started.</p>
            <Button className="mt-4" onClick={openCreate}>Create room</Button>
          </div>
        ) : null}
        {rooms.length > 0 ? (
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {rooms.map((room) => (
              <RoomCard
                key={room.id}
                room={roomDtoToRoom(room)}
                onJoin={() => void handleJoinPublic(room)}
              />
            ))}
          </div>
        ) : null}
      </div>

      {showCreate ? (
        <Modal
          title="Create room"
          eyebrow="New table"
          description="Pick how players join and how long each turn lasts."
          onClose={() => setShowCreate(false)}
        >
          <form onSubmit={handleCreateRoom} className="grid gap-5">
            <div className="grid gap-2">
              <span className="text-xs font-medium uppercase text-spade-gray-2">Visibility</span>
              <div role="group" aria-label="Visibility" className="grid grid-cols-2 gap-2">
                {(['public', 'private'] as const).map((value) => (
                  <button
                    key={value}
                    type="button"
                    aria-pressed={visibility === value}
                    onClick={() => setVisibility(value)}
                    className={`rounded-spade-md border px-3 py-2 text-sm font-medium capitalize transition ${
                      visibility === value
                        ? 'border-spade-gold bg-spade-gold/15 text-spade-gold-light'
                        : 'border-spade-cream/15 bg-spade-bg text-spade-gray-2 hover:border-spade-cream/30'
                    }`}
                  >
                    {value}
                  </button>
                ))}
              </div>
            </div>

            <div className="grid gap-2">
              <span className="text-xs font-medium uppercase text-spade-gray-2">Turn timer</span>
              <div role="group" aria-label="Turn timer" className="grid grid-cols-4 gap-2">
                {TIMER_OPTIONS.map((value) => (
                  <button
                    key={value}
                    type="button"
                    aria-pressed={timer === value}
                    onClick={() => setTimer(value)}
                    className={`rounded-spade-md border px-2 py-2 text-sm font-medium transition ${
                      timer === value
                        ? 'border-spade-gold bg-spade-gold/15 text-spade-gold-light'
                        : 'border-spade-cream/15 bg-spade-bg text-spade-gray-2 hover:border-spade-cream/30'
                    }`}
                  >
                    {value}s
                  </button>
                ))}
              </div>
            </div>

            {createError ? (
              <p role="alert" className="text-xs text-spade-red">
                {createError}
              </p>
            ) : null}

            <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <Button type="button" variant="secondary" onClick={() => setShowCreate(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={isCreating}>
                {isCreating ? 'Creating…' : 'Create'}
              </Button>
            </div>
          </form>
        </Modal>
      ) : null}

      {showJoin ? (
        <Modal
          title="Join by code"
          eyebrow="Private room"
          description="Enter the invite code shared by the host."
          onClose={() => setShowJoin(false)}
        >
          <form onSubmit={handleJoinByCode} className="grid gap-4">
            <label className="grid gap-1.5 text-xs font-medium uppercase text-spade-gray-2">
              Invite code
              <input
                value={inviteCode}
                onChange={(event) => setInviteCode(event.target.value.toUpperCase())}
                className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 font-mono text-sm tracking-[0.12em] text-spade-cream outline-none placeholder:text-spade-gray-3/60 focus:border-spade-gold focus:ring-2 focus:ring-spade-gold/20"
                placeholder="XKQP7A"
                maxLength={8}
                autoFocus
              />
            </label>
            {joinError ? (
              <p role="alert" className="text-xs text-spade-red">
                {joinError}
              </p>
            ) : null}
            <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <Button type="button" variant="secondary" onClick={() => setShowJoin(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={isJoining}>
                {isJoining ? 'Joining…' : 'Join with code'}
              </Button>
            </div>
          </form>
        </Modal>
      ) : null}
    </SceneShell>
  )
}
