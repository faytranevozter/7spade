import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { Avatar } from '../components/Avatar'
import { EmoteBubble } from '../components/EmoteBubble'
import { EmotePicker } from '../components/EmotePicker'
import { SceneShell } from '../components/SceneShell'
import { ToastStack } from '../components/ToastStack'
import { ApiError } from '../api/client'
import { getRoom, type RoomDto } from '../api/lobby'
import { useAuth } from '../hooks/useAuth'
import { useGameSocket } from '../hooks/useGameSocket'
import { useActiveRoom } from '../hooks/useActiveRoom'
import { useSound } from '../hooks/useSound'
import { getTeamColor } from '../game/teams'
import { initialsForName } from '../game/cards'
import type { Toast } from '../types'

const connectionTone = {
  idle: 'waiting',
  connecting: 'waiting',
  open: 'playing',
  closed: 'danger',
  error: 'danger',
} as const

function getErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return fallback
}

export function WaitingRoomPage() {
  const { roomId } = useParams()
  const navigate = useNavigate()
  const { token, isAuthenticated } = useAuth()
  const game = useGameSocket(roomId, token)
  const { unlock: unlockSound } = useSound()
  const { refresh: refreshActiveRoom, clear: clearActiveRoom } = useActiveRoom()
  const [roomDetails, setRoomDetails] = useState<RoomDto | null>(null)
  const [pageToasts, setPageToasts] = useState<Toast[]>([])
  const pageToastIdRef = useRef(0)
  const [copied, setCopied] = useState(false)
  const [linkCopied, setLinkCopied] = useState(false)

  // pushPageToast surfaces a page-local failure (e.g. failing to load the room
  // detail) as a transient toast rather than persistent red text.
  const pushPageToast = useCallback((toast: Omit<Toast, 'id'>) => {
    const id = ++pageToastIdRef.current
    setPageToasts((current) => [{ ...toast, id }, ...current].slice(0, 3))
    window.setTimeout(() => {
      setPageToasts((current) => current.filter((t) => t.id !== id))
    }, 4000)
  }, [])

  useEffect(() => {
    if (!isAuthenticated) {
      navigate('/auth', { replace: true })
    }
  }, [isAuthenticated, navigate])

  useEffect(() => {
    if (!roomId || !token) return
    let cancelled = false
    getRoom(token, roomId)
      .then((data) => {
        if (!cancelled) setRoomDetails(data)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        // A 404 means the room no longer exists (e.g. it was deleted once the
        // last player left). Don't let the player linger in a phantom room —
        // send them back to the lobby instead of showing an inline error.
        if (err instanceof ApiError && err.statusCode === 404) {
          navigate('/lobby', { replace: true })
          return
        }
        pushPageToast({ tone: 'error', title: 'Could not load room', body: getErrorMessage(err, 'Failed to load room') })
      })
    return () => {
      cancelled = true
    }
  }, [roomId, token, navigate, pushPageToast])

  // Once the game starts the WS hook flips phase to 'playing'. Redirect to the
  // live game page so the existing socket can hand off cleanly via re-mount.
  useEffect(() => {
    if (game.phase === 'playing' && roomId) {
      navigate(`/game/${roomId}`, { replace: true })
    }
  }, [game.phase, roomId, navigate])

  // Refresh the app-wide active-game indicator when we enter a room so it picks
  // up this room promptly (the poll alone could lag a few seconds).
  useEffect(() => {
    refreshActiveRoom()
  }, [roomId, refreshActiveRoom])

  // The host kicked us (room_closed): clear the active-game indicator and head
  // back to the main lobby.
  useEffect(() => {
    if (game.roomClosed) {
      clearActiveRoom()
      navigate('/lobby', { replace: true })
    }
  }, [game.roomClosed, clearActiveRoom, navigate])

  const lobby = game.lobby
  // Count only connected players for the live "X / N" badge; disconnected
  // players (within the reconnect grace window) are still shown as held seats
  // below but don't count toward the active total.
  const playerCount = lobby?.players.filter((p) => !p.disconnected).length ?? 0
  const minToStart = lobby?.minToStart ?? 2
  const maxPlayers = lobby?.maxPlayers ?? 4
  const slots = useMemo(() => {
    const filled = lobby?.players ?? []
    const placeholders: Array<null> = Array.from({ length: Math.max(0, maxPlayers - filled.length) }, () => null)
    return [...filled, ...placeholders]
  }, [lobby?.players, maxPlayers])

  const startBlockedReason = (() => {
    if (!lobby) return 'Connecting…'
    if (playerCount < minToStart) return `Need at least ${minToStart} players`
    if (!lobby.canStart) return 'Waiting for everyone to ready up'
    return null
  })()

  const inviteCode = roomDetails?.invite_code ?? ''
  // Prefer the live socket flag once connected, but fall back to the REST room
  // detail so the badge/copy render correctly before the first lobby_state.
  const practiceMode = game.practiceMode || Boolean(roomDetails?.practice_mode)
  const handleCopyCode = async () => {
    if (!inviteCode) return
    try {
      await navigator.clipboard.writeText(inviteCode)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      // Clipboard API is best-effort; fall back silently.
    }
  }

  const handleCopyLink = async () => {
    if (!inviteCode) return
    // A shareable link a friend can open to land on the lobby with the join
    // dialog pre-filled (LobbyPage reads ?invite=).
    const link = `${window.location.origin}/lobby?invite=${encodeURIComponent(inviteCode)}`
    try {
      await navigator.clipboard.writeText(link)
      setLinkCopied(true)
      window.setTimeout(() => setLinkCopied(false), 1500)
    } catch {
      // Best-effort.
    }
  }

  const handleLeave = () => {
    // Tell the server we're leaving so other players see the seat free up
    // immediately (no reconnect-grace delay), then navigate away. Clear the
    // active-game indicator optimistically since we're no longer in this room.
    game.sendLeave()
    clearActiveRoom()
    refreshActiveRoom()
    navigate('/lobby')
  }

  const action = (
    <div className="flex flex-wrap gap-2">
      <Badge tone={game.status === 'open' ? 'playing' : connectionTone[game.status]}>
        {game.status === 'open' ? 'Connected' : game.status}
      </Badge>
      <Badge tone="waiting">{`${playerCount} / ${maxPlayers} players`}</Badge>
      {practiceMode ? (
        <Badge tone="winner">Practice</Badge>
      ) : roomDetails?.visibility ? (
        <Badge tone="waiting">{roomDetails.visibility === 'private' ? 'Private' : 'Public'}</Badge>
      ) : null}
    </div>
  )

  return (
    <SceneShell title={roomDetails?.name || 'Waiting room'} eyebrow="Pre-game lobby" action={action}>
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_340px]">
        <div className="grid gap-4">
          <div className="rounded-spade-lg border border-spade-cream/10 bg-spade-bg/55 p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h3 className="text-lg font-medium">{practiceMode ? 'Practice' : 'Invite'}</h3>
                <p className="mt-1 text-sm text-spade-gray-2">
                  {practiceMode
                    ? 'Solo practice vs three bots. Start whenever you like — this game is not saved to history or stats.'
                    : `Share this code so up to ${maxPlayers} players can join. The host can start with at least ${minToStart}; remaining seats fill with bots.`}
                </p>
              </div>
              {roomDetails?.turn_timer_seconds ? (
                <Badge tone="waiting">{`Turn timer: ${roomDetails.turn_timer_seconds}s`}</Badge>
              ) : null}
              {roomDetails?.bot_difficulty ? (
                <Badge tone="waiting">{`Bots: ${roomDetails.bot_difficulty}`}</Badge>
              ) : null}
            </div>
            <div className="mt-4 flex flex-wrap items-center gap-3">
              <code className="rounded-spade-md border border-spade-gold/40 bg-spade-gold/10 px-4 py-2 font-mono text-lg tracking-[0.2em] text-spade-gold-light">
                {inviteCode || '······'}
              </code>
              {!practiceMode ? (
                <>
                  <Button variant="secondary" onClick={handleCopyCode} disabled={!inviteCode}>
                    {copied ? 'Copied' : 'Copy code'}
                  </Button>
                  <Button variant="secondary" onClick={handleCopyLink} disabled={!inviteCode}>
                    {linkCopied ? 'Link copied' : 'Invite a friend'}
                  </Button>
                </>
              ) : null}
            </div>
          </div>

          {roomDetails ? (
            <div className="rounded-spade-lg border border-spade-gold/20 bg-spade-gold/5 p-4">
              <h3 className="text-lg font-medium text-spade-gold-light">Game rules</h3>
              <div className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
                <div className="grid gap-0.5">
                  <span className="text-[10px] font-medium uppercase text-spade-gray-3">Players</span>
                  <span className="text-sm font-medium text-spade-cream">{roomDetails.max_players}</span>
                </div>
                <div className="grid gap-0.5">
                  <span className="text-[10px] font-medium uppercase text-spade-gray-3">Deck</span>
                  <span className="text-sm font-medium text-spade-cream">{roomDetails.deck_count === 2 ? 'Double (104)' : 'Single (52)'}</span>
                </div>
                <div className="grid gap-0.5">
                  <span className="text-[10px] font-medium uppercase text-spade-gray-3">Scoring</span>
                  <span className="text-sm font-medium text-spade-cream">{roomDetails.scoring_mode === 'flat' ? 'Flat (1pt)' : roomDetails.scoring_mode === 'custom' ? 'Custom' : 'Classic'}</span>
                </div>
                <div className="grid gap-0.5">
                  <span className="text-[10px] font-medium uppercase text-spade-gray-3">Teams</span>
                  <span className="text-sm font-medium text-spade-cream">{roomDetails.team_mode === '2v2' ? '2v2 Teams' : 'Free for All'}</span>
                </div>
              </div>
            </div>
          ) : null}

          <div className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
            <h3 className="text-lg font-medium">Players</h3>
            <p className="mt-1 text-sm text-spade-gray-2">
              The host is auto-ready. Everyone else needs to mark themselves ready before the game can start.
            </p>
            <ul className="mt-4 grid gap-2" aria-label="Players in waiting room">
              {slots.map((player, index) => (
                <li
								key={player ? `slot-${player.slot}` : `empty-${index}`}
                  className={`flex items-center justify-between gap-3 rounded-spade-md border border-spade-cream/10 bg-spade-bg/55 px-3 py-2 ${player?.disconnected ? 'opacity-55' : ''}`}
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <span className="relative inline-grid">
                      {player ? (
                        <EmoteBubble
                          emote={game.emotes[player.displayName]}
                          placementClassName="-right-2 -top-2 translate-x-1/2 -translate-y-1/2"
                        />
                      ) : null}
                      {player ? (
                        <Avatar avatarUrl={player.avatarUrl} initials={initialsForName(player.displayName)} tone="green" sizeClass="size-9" />
                      ) : (
                        <span className="grid size-9 place-items-center rounded-full bg-spade-green-mid text-sm font-medium text-spade-cream">—</span>
                      )}
                    </span>
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">
                        {player ? player.displayName : 'Waiting for player…'}
                      </p>
                      <p className="font-mono text-[11px] text-spade-gray-3">
                        {player ? `Slot ${index + 1}` : `Slot ${index + 1} · open`}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    {player?.isHost ? <Badge tone="winner">Host</Badge> : null}
                    {player && lobby?.teamMode === '2v2' ? (
                      <span className={`inline-flex items-center gap-1.5 rounded-spade-pill border px-3 py-1 text-[11px] font-medium before:block before:size-1.5 before:rounded-full ${getTeamColor(player.team ?? 0).badge}`}>
                        Team {(player.team ?? 0) + 1}
                      </span>
                    ) : null}
                    {player ? (
                      player.disconnected ? (
                        <Badge tone="danger">Disconnected</Badge>
                      ) : (
                        <Badge tone={player.ready ? 'playing' : 'waiting'}>
                          {player.ready ? 'Ready' : 'Not ready'}
                        </Badge>
                      )
                    ) : null}
                    {player && game.isHost && !player.isHost ? (
                      <button
                        type="button"
                        onClick={() => game.sendKick(player.slot)}
                        aria-label={`Remove ${player.displayName} from the room`}
                        title="Kick player"
                        className="grid size-7 place-items-center rounded-full text-sm transition hover:bg-spade-red/15 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-spade-red/40"
                      >
                        <span aria-hidden="true">👢</span>
                      </button>
                    ) : null}
                  </div>
                </li>
              ))}
            </ul>
          </div>

          <ToastStack toasts={[...pageToasts, ...game.toasts]} />
        </div>

        <div className="grid content-start gap-3 rounded-spade-lg border border-spade-cream/10 bg-spade-bg/55 p-4">
          <h3 className="text-lg font-medium">Your status</h3>
          {game.isHost ? (
            <>
              <p className="text-sm text-spade-gray-2">
                {practiceMode
                  ? "You're practicing solo. The other three seats are bots — start whenever you're ready."
                  : "You're the host. Empty seats will be filled with bots when the game starts."}
              </p>
              <Button onClick={() => { unlockSound(); game.sendStartGame() }} disabled={!lobby?.canStart}>
                {practiceMode ? 'Start practice' : 'Start game'}
              </Button>
              {startBlockedReason ? (
                <p className="font-mono text-[11px] text-spade-gray-3">{startBlockedReason}</p>
              ) : null}
            </>
          ) : (
            <>
              <p className="text-sm text-spade-gray-2">
                Mark yourself ready when you're set. The host will start the round once everyone is ready.
              </p>
              <Button onClick={() => { unlockSound(); game.sendSetReady(!game.iAmReady) }}>
                {game.iAmReady ? 'Cancel ready' : 'Mark ready'}
              </Button>
              <p className="font-mono text-[11px] text-spade-gray-3">
                {game.iAmReady ? 'Waiting for the host to start.' : 'Tell the host you are ready.'}
              </p>
            </>
          )}
          <Button variant="danger" onClick={handleLeave}>
            Leave room
          </Button>
          {lobby?.teamMode === '2v2' ? (
            <div className="border-t border-spade-cream/10 pt-3">
              <span className="text-sm font-medium text-spade-gray-2">Choose your team</span>
              <div className="mt-2 grid grid-cols-3 gap-2">
                {Array.from({ length: (lobby.maxPlayers ?? 4) / 2 }, (_, i) => i).map((team) => {
                  const myTeam = lobby.players.find((p) => p.displayName === game.myDisplayName)?.team
                  const teamCount = lobby.players.filter((p) => p.team === team).length
                  const isMine = myTeam === team
                  const isFull = !isMine && teamCount >= 2
                  return (
                    <button
                      key={team}
                      type="button"
                      disabled={isFull}
                      onClick={() => game.sendSetTeam(team)}
                      className={`inline-flex items-center justify-center gap-1.5 rounded-spade-pill border px-3 py-2 text-xs font-medium transition before:block before:size-1.5 before:rounded-full ${
                        isFull
                          ? 'border-spade-cream/10 bg-spade-bg text-spade-gray-3/50 before:bg-spade-gray-3/50 cursor-not-allowed'
                          : isMine
                            ? getTeamColor(team).badge
                            : 'border-spade-cream/15 bg-spade-bg text-spade-gray-2 before:bg-spade-gray-3 hover:border-spade-cream/30'
                      }`}
                    >
                      Team {team + 1} ({teamCount}/2)
                    </button>
                  )
                })}
              </div>
            </div>
          ) : null}
          <div className="flex items-center justify-between gap-3 border-t border-spade-cream/10 pt-3">
            <span className="text-sm text-spade-gray-2">Send an emote</span>
            <EmotePicker onSelect={game.sendEmote} />
          </div>
        </div>
      </div>
    </SceneShell>
  )
}
