import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { SceneShell } from '../components/SceneShell'
import { ToastStack } from '../components/ToastStack'
import { ApiError } from '../api/client'
import { getRoom, type RoomDto } from '../api/lobby'
import { useAuth } from '../hooks/useAuth'
import { useGameSocket } from '../hooks/useGameSocket'
import { initialsForName } from '../game/cards'

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
  const [roomDetails, setRoomDetails] = useState<RoomDto | null>(null)
  const [roomError, setRoomError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

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
        setRoomError(getErrorMessage(err, 'Failed to load room'))
      })
    return () => {
      cancelled = true
    }
  }, [roomId, token, navigate])

  // Once the game starts the WS hook flips phase to 'playing'. Redirect to the
  // live game page so the existing socket can hand off cleanly via re-mount.
  useEffect(() => {
    if (game.phase === 'playing' && roomId) {
      navigate(`/game/${roomId}`, { replace: true })
    }
  }, [game.phase, roomId, navigate])

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

  const handleLeave = () => {
    // Tell the server we're leaving so other players see the seat free up
    // immediately (no reconnect-grace delay), then navigate away.
    game.sendLeave()
    navigate('/lobby')
  }

  const action = (
    <div className="flex flex-wrap gap-2">
      <Badge tone={game.status === 'open' ? 'playing' : connectionTone[game.status]}>
        {game.status === 'open' ? 'Connected' : game.status}
      </Badge>
      <Badge tone="waiting">{`${playerCount} / ${maxPlayers} players`}</Badge>
      {roomDetails?.visibility ? (
        <Badge tone="waiting">{roomDetails.visibility === 'private' ? 'Private' : 'Public'}</Badge>
      ) : null}
    </div>
  )

  return (
    <SceneShell title="Waiting room" eyebrow="Pre-game lobby" action={action}>
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_340px]">
        <div className="grid gap-4">
          <div className="rounded-spade-lg border border-spade-cream/10 bg-spade-bg/55 p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h3 className="text-lg font-medium">Invite</h3>
                <p className="mt-1 text-sm text-spade-gray-2">
                  Share this code so up to {maxPlayers} players can join. The host can start with at least {minToStart}; remaining seats fill with bots.
                </p>
              </div>
              {roomDetails?.turn_timer_seconds ? (
                <Badge tone="waiting">{`Turn timer: ${roomDetails.turn_timer_seconds}s`}</Badge>
              ) : null}
            </div>
            <div className="mt-4 flex flex-wrap items-center gap-3">
              <code className="rounded-spade-md border border-spade-gold/40 bg-spade-gold/10 px-4 py-2 font-mono text-lg tracking-[0.2em] text-spade-gold-light">
                {inviteCode || '······'}
              </code>
              <Button variant="secondary" onClick={handleCopyCode} disabled={!inviteCode}>
                {copied ? 'Copied' : 'Copy code'}
              </Button>
            </div>
            {roomError ? (
              <p role="alert" className="mt-3 text-xs text-spade-red">
                {roomError}
              </p>
            ) : null}
          </div>

          <div className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
            <h3 className="text-lg font-medium">Players</h3>
            <p className="mt-1 text-sm text-spade-gray-2">
              The host is auto-ready. Everyone else needs to mark themselves ready before the game can start.
            </p>
            <ul className="mt-4 grid gap-2" aria-label="Players in waiting room">
              {slots.map((player, index) => (
                <li
                  key={player ? player.displayName : `empty-${index}`}
                  className={`flex items-center justify-between gap-3 rounded-spade-md border border-spade-cream/10 bg-spade-bg/55 px-3 py-2 ${player?.disconnected ? 'opacity-55' : ''}`}
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <span className="grid size-9 place-items-center rounded-full bg-spade-green-mid text-sm font-medium text-spade-cream">
                      {player ? initialsForName(player.displayName) : '—'}
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
                    {player ? (
                      player.disconnected ? (
                        <Badge tone="danger">Disconnected</Badge>
                      ) : (
                        <Badge tone={player.ready ? 'playing' : 'waiting'}>
                          {player.ready ? 'Ready' : 'Not ready'}
                        </Badge>
                      )
                    ) : null}
                  </div>
                </li>
              ))}
            </ul>
          </div>

          <ToastStack toasts={game.toasts} />
        </div>

        <div className="grid content-start gap-3 rounded-spade-lg border border-spade-cream/10 bg-spade-bg/55 p-4">
          <h3 className="text-lg font-medium">Your status</h3>
          {game.isHost ? (
            <>
              <p className="text-sm text-spade-gray-2">
                You're the host. Empty seats will be filled with bots when the game starts.
              </p>
              <Button onClick={game.sendStartGame} disabled={!lobby?.canStart}>
                Start game
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
              <Button onClick={() => game.sendSetReady(!game.iAmReady)}>
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
        </div>
      </div>
    </SceneShell>
  )
}
