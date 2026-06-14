import { type FormEvent, useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { Modal } from '../components/Modal'
import { RoomCard } from '../components/RoomCard'
import { SceneShell } from '../components/SceneShell'
import { ToastStack } from '../components/ToastStack'
import { ApiError } from '../api/client'
import {
  getRooms,
  postJoinRoom,
  postQuickPlay,
  postRoom,
  type BotDifficulty,
  type RoomDto,
  type RoomVisibility,
} from '../api/lobby'
import { getMyStats } from '../api/stats'
import { useAuth } from '../hooks/useAuth'
import { useActiveRoom } from '../hooks/useActiveRoom'
import { getLiveGames, type LiveGameDto } from '../api/liveGames'
import { FriendsPanel } from '../components/FriendsPanel'
import { decodeJwtClaims } from '../auth/claims'
import type { Room, Toast } from '../types'

const TIMER_OPTIONS: ReadonlyArray<30 | 60 | 90 | 120> = [30, 60, 90, 120]
const BOT_DIFFICULTY_OPTIONS: ReadonlyArray<BotDifficulty> = ['easy', 'medium', 'hard']
const TOAST_TTL_MS = 4000

function botDifficultyLabel(value: BotDifficulty): string {
  return value.charAt(0).toUpperCase() + value.slice(1)
}

function roomDtoToRoom(dto: RoomDto): Room {
  const fillStatus = dto.player_count >= 4 ? 'Full' : `${dto.player_count} / 4 players`
  const eloRange = dto.min_elo !== null && dto.max_elo !== null ? `ELO ${dto.min_elo}-${dto.max_elo}` : undefined
  return {
    name: dto.name || (dto.visibility === 'private' ? 'Private room' : 'Public room'),
    code: dto.invite_code,
    players: `${dto.player_count} / 4`,
    status: dto.status === 'waiting' ? fillStatus : `Status: ${dto.status}`,
    timer: `${dto.turn_timer_seconds}s`,
    botDifficulty: botDifficultyLabel(dto.bot_difficulty),
    eloRange,
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
  const { refresh: refreshActiveRoom } = useActiveRoom()
  const isGuest = decodeJwtClaims(token).isGuest

  const [rooms, setRooms] = useState<RoomDto[]>([])
  const [isLoadingRooms, setIsLoadingRooms] = useState(false)
  const [listError, setListError] = useState<string | null>(null)
  const [liveGames, setLiveGames] = useState<LiveGameDto[]>([])

  const [visibility, setVisibility] = useState<RoomVisibility>('public')
  const [roomName, setRoomName] = useState('')
  const [timer, setTimer] = useState<30 | 60 | 90 | 120>(60)
  const [botDifficulty, setBotDifficulty] = useState<BotDifficulty>('medium')
  const [limitByRating, setLimitByRating] = useState(false)
  const [minElo, setMinElo] = useState(1000)
  const [maxElo, setMaxElo] = useState(1400)
  const [isCreating, setIsCreating] = useState(false)
  const [showCreate, setShowCreate] = useState(false)

  const [inviteCode, setInviteCode] = useState('')
  const [isJoining, setIsJoining] = useState(false)
  // joinError is reserved for inline form validation in the join modal (e.g. an
  // empty code); server/action failures surface as toasts instead.
  const [joinError, setJoinError] = useState<string | null>(null)
  const [showJoin, setShowJoin] = useState(false)

  const [showPractice, setShowPractice] = useState(false)
  const [practiceTimer, setPracticeTimer] = useState<30 | 60 | 90 | 120>(60)
  const [practiceBotDifficulty, setPracticeBotDifficulty] = useState<BotDifficulty>('medium')
  const [isStartingPractice, setIsStartingPractice] = useState(false)

  const [isQuickPlaying, setIsQuickPlaying] = useState(false)
  const [isRankedQuickPlaying, setIsRankedQuickPlaying] = useState(false)
  const [toasts, setToasts] = useState<Toast[]>([])
  const [myRating, setMyRating] = useState<number | null>(null)
  const toastIdRef = useRef(0)

  const [refreshNonce, setRefreshNonce] = useState(0)
  const [searchParams, setSearchParams] = useSearchParams()

  // pushToast surfaces a transient notification for any lobby action failure
  // (join, create, practice, quick play). Capped and auto-dismissed.
  const pushToast = useCallback((toast: Omit<Toast, 'id'>) => {
    const id = ++toastIdRef.current
    setToasts((current) => [{ ...toast, id }, ...current].slice(0, 3))
    window.setTimeout(() => {
      setToasts((current) => current.filter((t) => t.id !== id))
    }, TOAST_TTL_MS)
  }, [])

  // When a join/create/quick-play is rejected because the player is already in
  // another active game, take them straight to that game instead of showing a
  // dead-end error. Returns true when it handled the error.
  const redirectToActiveRoomOnConflict = useCallback((err: unknown): boolean => {
    if (err instanceof ApiError && err.activeRoom) {
      const room = err.activeRoom
      refreshActiveRoom()
      navigate(room.status === 'in_progress' ? `/game/${room.id}` : `/room/${room.id}`)
      return true
    }
    return false
  }, [navigate, refreshActiveRoom])

  useEffect(() => {
    if (!isAuthenticated) {
      // Preserve an invite across the sign-in redirect so a friend opening
      // /lobby?invite=CODE while logged out still lands on the join dialog.
      const invite = searchParams.get('invite')
      if (invite) {
        try {
          sessionStorage.setItem('seven_spade_pending_invite', invite)
        } catch {
          // Best-effort.
        }
      }
      navigate('/auth', { replace: true })
    }
  }, [isAuthenticated, navigate, searchParams])

  // An invite link (/lobby?invite=CODE) — or one stashed across the sign-in
  // redirect — prefills the join dialog so a friend can jump straight in.
  // Consumed once so a refresh doesn't reopen it.
  useEffect(() => {
    if (!isAuthenticated) return
    let invite = searchParams.get('invite')
    let fromStash = false
    if (!invite) {
      try {
        invite = sessionStorage.getItem('seven_spade_pending_invite')
        fromStash = invite !== null
      } catch {
        invite = null
      }
    }
    if (!invite) return
    const code = invite
    // Defer the state writes out of the effect body (avoids set-state-in-effect).
    const id = window.setTimeout(() => {
      setInviteCode(code.toUpperCase())
      setShowJoin(true)
      if (fromStash) {
        try {
          sessionStorage.removeItem('seven_spade_pending_invite')
        } catch {
          // Best-effort.
        }
      } else {
        const next = new URLSearchParams(searchParams)
        next.delete('invite')
        setSearchParams(next, { replace: true })
      }
    }, 0)
    return () => window.clearTimeout(id)
  }, [isAuthenticated, searchParams, setSearchParams])

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

  useEffect(() => {
    if (!isAuthenticated || isGuest) return
    let cancelled = false
    getMyStats(token)
      .then((stats) => {
        if (!cancelled) setMyRating(stats.rating)
      })
      .catch(() => {
        if (!cancelled) setMyRating(null)
      })
    return () => {
      cancelled = true
    }
  }, [isAuthenticated, isGuest, token])

  // Load in-progress public games to watch, on the same cadence as the room
  // list. Failures are non-fatal: the watch section just stays empty.
  const loadLiveGames = useCallback(() => {
    if (!isAuthenticated) return () => {}
    let cancelled = false
    getLiveGames(token)
      .then((data) => {
        if (!cancelled) setLiveGames(data.games)
      })
      .catch(() => {
        // Non-fatal; leave the watch section empty.
      })
    return () => {
      cancelled = true
    }
  }, [isAuthenticated, token])

  useEffect(() => loadLiveGames(), [loadLiveGames, refreshNonce])

  // Auto-refresh the list so rooms that fill up, start, or get deleted (e.g.
  // when their last player leaves) drop off without a manual refresh.
  useEffect(() => {
    if (!isAuthenticated) return
    const interval = window.setInterval(() => {
      loadRooms(true)
      loadLiveGames()
    }, 5000)
    return () => window.clearInterval(interval)
  }, [isAuthenticated, loadRooms, loadLiveGames])

  const refreshRooms = useCallback(() => {
    setRefreshNonce((n) => n + 1)
  }, [])

  const handleCreateRoom = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setIsCreating(true)
    try {
      const created = await postRoom(token, {
        ...(roomName.trim() ? { name: roomName.trim() } : {}),
        visibility,
        turn_timer_seconds: timer,
        bot_difficulty: botDifficulty,
        ...(limitByRating && visibility === 'public' ? { min_elo: minElo, max_elo: maxElo } : {}),
      })
      navigate(`/room/${created.id}`)
    } catch (err) {
      if (redirectToActiveRoomOnConflict(err)) return
      pushToast({ tone: 'error', title: 'Could not create room', body: getErrorMessage(err, 'Failed to create room') })
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
      if (redirectToActiveRoomOnConflict(err)) return
      pushToast({ tone: 'error', title: 'Could not join room', body: getErrorMessage(err, 'Failed to join room') })
    } finally {
      setIsJoining(false)
    }
  }

  const handleJoinPublic = async (room: RoomDto) => {
    try {
      const joined = await postJoinRoom(token, room.invite_code)
      navigate(`/room/${joined.id}`)
    } catch (err) {
      if (redirectToActiveRoomOnConflict(err)) return
      pushToast({ tone: 'error', title: 'Could not join room', body: getErrorMessage(err, 'Failed to join room') })
    }
  }

  const handleQuickPlay = async () => {
    setIsQuickPlaying(true)
    try {
      const joined = await postQuickPlay(token)
      navigate(`/room/${joined.id}`)
    } catch (err) {
      if (redirectToActiveRoomOnConflict(err)) return
      pushToast({ tone: 'error', title: 'Quick Play failed', body: getErrorMessage(err, 'Failed to find a game') })
    } finally {
      setIsQuickPlaying(false)
    }
  }

  const handleRankedQuickPlay = async () => {
    if (isGuest) {
      pushToast({ tone: 'error', title: 'Ranked Quick Play unavailable', body: 'Sign in to use rating-based matchmaking.' })
      return
    }
    setIsRankedQuickPlaying(true)
    try {
      const joined = await postQuickPlay(token, { ranked: true })
      navigate(`/room/${joined.id}`)
    } catch (err) {
      if (redirectToActiveRoomOnConflict(err)) return
      pushToast({ tone: 'error', title: 'Ranked Quick Play failed', body: getErrorMessage(err, 'Failed to find a ranked game') })
    } finally {
      setIsRankedQuickPlaying(false)
    }
  }

  const openCreate = () => {
    setRoomName('')
    if (myRating !== null) {
      setMinElo(Math.max(0, myRating - 200))
      setMaxElo(myRating + 200)
    }
    setShowCreate(true)
  }

  const handleStartPractice = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setIsStartingPractice(true)
    try {
      const created = await postRoom(token, {
        visibility: 'private',
        turn_timer_seconds: practiceTimer,
        bot_difficulty: practiceBotDifficulty,
        practice_mode: true,
      })
      navigate(`/room/${created.id}`)
    } catch (err) {
      if (redirectToActiveRoomOnConflict(err)) return
      pushToast({ tone: 'error', title: 'Could not start practice', body: getErrorMessage(err, 'Failed to start practice') })
    } finally {
      setIsStartingPractice(false)
    }
  }

  const openPractice = () => {
    setShowPractice(true)
  }

  const openJoin = () => {
    setJoinError(null)
    setShowJoin(true)
  }

  const openRoomCount = rooms.filter((room) => room.status === 'waiting' && room.player_count < 4).length
  const isConstrainedRoom = (room: RoomDto) => room.min_elo !== null && room.max_elo !== null
  const matchesMyRating = (room: RoomDto) => myRating !== null && room.min_elo !== null && room.max_elo !== null && myRating >= room.min_elo && myRating <= room.max_elo
  const ratingMatchedRooms = rooms.filter((room) => isConstrainedRoom(room) && matchesMyRating(room))
  const openRooms = rooms.filter((room) => !isConstrainedRoom(room))

  return (
    <SceneShell
      title="Game lobby"
      eyebrow="Lobby"
      action={
        <div className="flex flex-wrap items-center gap-2">
          <Badge tone="waiting">{`${openRoomCount} waiting`}</Badge>
          <Button onClick={() => void handleQuickPlay()} disabled={isQuickPlaying}>
            {isQuickPlaying ? 'Finding game…' : 'Quick Play'}
          </Button>
          <Button variant="ghost" onClick={() => void handleRankedQuickPlay()} disabled={isRankedQuickPlaying || isGuest}>
            {isRankedQuickPlaying ? 'Finding ranked game…' : 'Ranked Quick Play'}
          </Button>
          <Button onClick={openPractice}>Practice</Button>
          <Button variant="secondary" onClick={openCreate}>Create room</Button>
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
        {toasts.length > 0 ? (
          <div role="alert">
            <ToastStack toasts={toasts} />
          </div>
        ) : null}
        {!isLoadingRooms && rooms.length === 0 && !listError ? (
          <div className="rounded-spade-lg border border-dashed border-spade-cream/15 bg-spade-bg/40 p-10 text-center">
            <p className="text-sm text-spade-gray-2">No public rooms waiting.</p>
            <p className="mt-1 text-xs text-spade-gray-3">Create one to get the table started.</p>
            <Button className="mt-4" onClick={openCreate}>Create room</Button>
          </div>
        ) : null}
        {ratingMatchedRooms.length > 0 ? (
          <section className="grid gap-3">
            <h3 className="text-sm font-medium text-spade-gray-2">Rating-matched rooms</h3>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {ratingMatchedRooms.map((room) => (
                <RoomCard key={room.id} room={roomDtoToRoom(room)} onJoin={() => void handleJoinPublic(room)} />
              ))}
            </div>
          </section>
        ) : null}
        {openRooms.length > 0 ? (
          <section className="grid gap-3">
            <h3 className="text-sm font-medium text-spade-gray-2">Open rooms</h3>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {openRooms.map((room) => (
                <RoomCard key={room.id} room={roomDtoToRoom(room)} onJoin={() => void handleJoinPublic(room)} />
              ))}
            </div>
          </section>
        ) : null}

        {liveGames.length > 0 ? (
          <div className="mt-4 grid gap-3">
            <h3 className="text-sm font-medium text-spade-gray-2">Watch live</h3>
            <div className="grid gap-2">
              {liveGames.map((live) => (
                <div
                  key={live.room_id}
                  className="flex items-center justify-between gap-3 rounded-spade-lg border border-spade-cream/10 bg-spade-bg/55 px-3 py-2"
                >
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium text-spade-cream">
                      {live.players.map((p) => p.display_name).join(', ') || 'In progress'}
                    </p>
                    <p className="font-mono text-[11px] text-spade-gray-3">
                      {live.player_count} {live.player_count === 1 ? 'player' : 'players'} · in progress
                    </p>
                  </div>
                  <Button variant="secondary" onClick={() => navigate(`/watch/${live.room_id}`)}>
                    Watch
                  </Button>
                </div>
              ))}
            </div>
          </div>
        ) : null}

        {!isGuest ? <FriendsPanel token={token} refreshNonce={refreshNonce} /> : null}
      </div>

      {showCreate ? (
        <Modal
          title="Create room"
          eyebrow="New table"
          description="Pick how players join and how long each turn lasts."
          onClose={() => setShowCreate(false)}
        >
          <form onSubmit={handleCreateRoom} className="grid gap-5">
            <label className="grid gap-2">
              <span className="text-xs font-medium uppercase text-spade-gray-2">Room name</span>
              <input
                type="text"
                value={roomName}
                onChange={(event) => setRoomName(event.target.value)}
                maxLength={60}
                placeholder="Leave blank for a default name (Room #…)"
                className="rounded-spade-md border border-spade-cream/15 bg-spade-bg px-3 py-2 text-sm text-spade-cream placeholder:text-spade-gray-3 focus:border-spade-gold focus:outline-none"
              />
            </label>

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

            <div className="grid gap-2">
              <span className="text-xs font-medium uppercase text-spade-gray-2">Bot difficulty</span>
              <div role="group" aria-label="Bot difficulty" className="grid grid-cols-3 gap-2">
                {BOT_DIFFICULTY_OPTIONS.map((value) => (
                  <button
                    key={value}
                    type="button"
                    aria-pressed={botDifficulty === value}
                    onClick={() => setBotDifficulty(value)}
                    className={`rounded-spade-md border px-2 py-2 text-sm font-medium capitalize transition ${
                      botDifficulty === value
                        ? 'border-spade-gold bg-spade-gold/15 text-spade-gold-light'
                        : 'border-spade-cream/15 bg-spade-bg text-spade-gray-2 hover:border-spade-cream/30'
                    }`}
                  >
                    {value}
                  </button>
                ))}
              </div>
            </div>

            {visibility === 'public' ? (
              <div className="grid gap-3 rounded-spade-md border border-spade-cream/10 bg-spade-bg/45 p-3">
                <label className="flex items-center gap-2 text-sm text-spade-gray-2">
                  <input
                    type="checkbox"
                    checked={limitByRating}
                    onChange={(event) => setLimitByRating(event.target.checked)}
                    className="size-4 accent-spade-gold"
                  />
                  Limit room by rating
                </label>
                {limitByRating ? (
                  <div className="grid gap-3 sm:grid-cols-2">
                    <label className="grid gap-1.5 text-xs font-medium uppercase text-spade-gray-2">
                      Min rating
                      <input
                        type="number"
                        min={0}
                        value={minElo}
                        onChange={(event) => setMinElo(Number(event.target.value))}
                        className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-2 text-sm text-spade-cream outline-none focus:border-spade-gold focus:ring-2 focus:ring-spade-gold/20"
                      />
                    </label>
                    <label className="grid gap-1.5 text-xs font-medium uppercase text-spade-gray-2">
                      Max rating
                      <input
                        type="number"
                        min={0}
                        value={maxElo}
                        onChange={(event) => setMaxElo(Number(event.target.value))}
                        className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-2 text-sm text-spade-cream outline-none focus:border-spade-gold focus:ring-2 focus:ring-spade-gold/20"
                      />
                    </label>
                  </div>
                ) : null}
              </div>
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

      {showPractice ? (
        <Modal
          title="Practice mode"
          eyebrow="Solo vs bots"
          description="Play a private game against three bots. Practice games are not saved to history or stats."
          onClose={() => setShowPractice(false)}
        >
          <form onSubmit={handleStartPractice} className="grid gap-5">
            <div className="grid gap-2">
              <span className="text-xs font-medium uppercase text-spade-gray-2">Bot difficulty</span>
              <div role="group" aria-label="Practice bot difficulty" className="grid grid-cols-3 gap-2">
                {BOT_DIFFICULTY_OPTIONS.map((value) => (
                  <button
                    key={value}
                    type="button"
                    aria-pressed={practiceBotDifficulty === value}
                    onClick={() => setPracticeBotDifficulty(value)}
                    className={`rounded-spade-md border px-2 py-2 text-sm font-medium capitalize transition ${
                      practiceBotDifficulty === value
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
              <div role="group" aria-label="Practice turn timer" className="grid grid-cols-4 gap-2">
                {TIMER_OPTIONS.map((value) => (
                  <button
                    key={value}
                    type="button"
                    aria-pressed={practiceTimer === value}
                    onClick={() => setPracticeTimer(value)}
                    className={`rounded-spade-md border px-2 py-2 text-sm font-medium transition ${
                      practiceTimer === value
                        ? 'border-spade-gold bg-spade-gold/15 text-spade-gold-light'
                        : 'border-spade-cream/15 bg-spade-bg text-spade-gray-2 hover:border-spade-cream/30'
                    }`}
                  >
                    {value}s
                  </button>
                ))}
              </div>
            </div>

            <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <Button type="button" variant="secondary" onClick={() => setShowPractice(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={isStartingPractice}>
                {isStartingPractice ? 'Starting…' : 'Start practice'}
              </Button>
            </div>
          </form>
        </Modal>
      ) : null}
    </SceneShell>
  )
}
