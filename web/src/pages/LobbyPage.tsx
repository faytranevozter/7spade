import { type FormEvent, useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
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

  const [inviteCode, setInviteCode] = useState('')
  const [isJoining, setIsJoining] = useState(false)
  const [joinError, setJoinError] = useState<string | null>(null)

  const [refreshNonce, setRefreshNonce] = useState(0)

  useEffect(() => {
    if (!isAuthenticated) {
      navigate('/auth', { replace: true })
    }
  }, [isAuthenticated, navigate])

  useEffect(() => {
    if (!isAuthenticated) return
    let cancelled = false
    Promise.resolve()
      .then(() => {
        if (cancelled) return null
        setIsLoadingRooms(true)
        setListError(null)
        return getRooms(token)
      })
      .then((data) => {
        if (cancelled || data === null) return
        setRooms(data)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setListError(getErrorMessage(err, 'Failed to load rooms'))
      })
      .finally(() => {
        if (cancelled) return
        setIsLoadingRooms(false)
      })
    return () => {
      cancelled = true
    }
  }, [isAuthenticated, token, refreshNonce])

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
      navigate(`/game/${created.id}`)
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
      navigate(`/game/${joined.id}`)
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
      navigate(`/game/${joined.id}`)
    } catch (err) {
      setJoinError(getErrorMessage(err, 'Failed to join room'))
    }
  }

  const openRoomCount = rooms.filter((room) => room.status === 'waiting' && room.player_count < 4).length

  return (
    <SceneShell
      title="Game lobby"
      eyebrow="Room creation + lobby"
      action={<Badge tone="waiting">{`${openRoomCount} waiting`}</Badge>}
    >
      <div className="grid gap-4 lg:grid-cols-[360px_minmax(0,1fr)]">
        <div className="grid content-start gap-4">
          <form
            onSubmit={handleCreateRoom}
            className="rounded-spade-lg border border-spade-green-light/25 bg-spade-bg/70 p-4"
          >
            <h3 className="text-lg font-medium">Create room</h3>
            <div className="mt-4 grid gap-3">
              <label className="grid gap-1 text-xs text-spade-gray-2">
                Visibility
                <select
                  value={visibility}
                  onChange={(event) => setVisibility(event.target.value as RoomVisibility)}
                  className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 text-sm text-spade-black"
                >
                  <option value="public">Public room</option>
                  <option value="private">Private invite code</option>
                </select>
              </label>
              <label className="grid gap-1 text-xs text-spade-gray-2">
                Turn timer
                <select
                  value={timer}
                  onChange={(event) => setTimer(Number(event.target.value) as 30 | 60 | 90 | 120)}
                  className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 text-sm text-spade-black"
                >
                  {TIMER_OPTIONS.map((value) => (
                    <option key={value} value={value}>
                      {value} seconds
                    </option>
                  ))}
                </select>
              </label>
              {createError ? (
                <p role="alert" className="text-xs text-spade-red">
                  {createError}
                </p>
              ) : null}
              <Button type="submit" disabled={isCreating}>
                {isCreating ? 'Creating…' : 'Create room'}
              </Button>
            </div>
          </form>

          <form
            onSubmit={handleJoinByCode}
            className="rounded-spade-lg border border-spade-green-light/25 bg-spade-bg/70 p-4"
          >
            <h3 className="text-lg font-medium">Join private room</h3>
            <div className="mt-4 grid gap-3">
              <label className="grid gap-1 text-xs text-spade-gray-2">
                Invite code
                <input
                  value={inviteCode}
                  onChange={(event) => setInviteCode(event.target.value.toUpperCase())}
                  className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 font-mono text-sm tracking-[0.08em] text-spade-black"
                  placeholder="XKQP7A"
                  maxLength={8}
                />
              </label>
              {joinError ? (
                <p role="alert" className="text-xs text-spade-red">
                  {joinError}
                </p>
              ) : null}
              <Button type="submit" disabled={isJoining}>
                {isJoining ? 'Joining…' : 'Join with code'}
              </Button>
            </div>
          </form>
        </div>

        <div className="grid content-start gap-2">
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
          {!isLoadingRooms && rooms.length === 0 && !listError ? (
            <p className="rounded-spade-lg border border-dashed border-spade-cream/15 bg-spade-bg/40 p-4 text-center text-xs text-spade-gray-3">
              No public rooms waiting. Create one to get started.
            </p>
          ) : null}
          {rooms.map((room) => (
            <RoomCard
              key={room.id}
              room={roomDtoToRoom(room)}
              onJoin={() => void handleJoinPublic(room)}
            />
          ))}
        </div>
      </div>
    </SceneShell>
  )
}
