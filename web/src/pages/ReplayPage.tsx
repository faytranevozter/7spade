import { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams, Link } from 'react-router'
import { GameBoard } from '../components/GameBoard'
import { Button } from '../components/Button'
import { SceneShell } from '../components/SceneShell'
import { useAuth } from '../hooks/useAuth'
import { getReplay } from '../api/replay'
import type { ReplayDto } from '../api/replay'
import { reconstructAt, rankLabel, replaySuitSymbol } from '../game/replay'
import type { ReplayState } from '../game/replay'
import { initialsForName } from '../game/cards'

const SPEEDS = [1, 2, 4] as const
type Speed = (typeof SPEEDS)[number]
const INTERVAL_MS: Record<Speed, number> = { 1: 1200, 2: 600, 4: 300 }

export function ReplayPage() {
  const { gameId } = useParams()
  const navigate = useNavigate()
  const { token, isAuthenticated } = useAuth()

  const [replay, setReplay] = useState<ReplayDto | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [index, setIndex] = useState(-1)
  const [playing, setPlaying] = useState(false)
  const [speed, setSpeed] = useState<Speed>(1)

  const moveListRef = useRef<HTMLDivElement>(null)
  const intervalRef = useRef<number | null>(null)

  useEffect(() => {
    if (!isAuthenticated) navigate('/auth', { replace: true })
  }, [isAuthenticated, navigate])

  useEffect(() => {
    if (!gameId || !token) return
    let cancelled = false
    getReplay(token, gameId)
      .then((data) => {
        if (cancelled) return
        setReplay(data)
        setIndex(-1)
      })
      .catch((err) => {
        if (cancelled) return
        setError(err?.message ?? 'Failed to load replay')
      })
      .finally(() => {
        if (cancelled) return
        setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [gameId, token])

  const totalMoves = replay?.moves.length ?? 0

  useEffect(() => {
    if (!playing || !replay) return
    intervalRef.current = window.setInterval(() => {
      setIndex((prev) => {
        if (prev >= totalMoves - 1) {
          setPlaying(false)
          return prev
        }
        return prev + 1
      })
    }, INTERVAL_MS[speed])
    return () => {
      if (intervalRef.current !== null) window.clearInterval(intervalRef.current)
    }
  }, [playing, speed, totalMoves, replay])

  // Scroll active move into view
  useEffect(() => {
    if (!moveListRef.current) return
    const active = moveListRef.current.querySelector('[data-active="true"]')
    active?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  }, [index])

  if (loading) {
    return (
      <SceneShell title="Replay" eyebrow="Loading…">
        <div className="py-12 text-center text-sm text-spade-gray-2">Loading replay…</div>
      </SceneShell>
    )
  }

  if (error || !replay) {
    return (
      <SceneShell title="Replay" eyebrow="Not available">
        <div className="grid gap-4 py-8 text-center">
          <p className="text-sm text-spade-gray-2">
            {error ?? 'Replay not available for this game.'}
          </p>
          <div className="flex justify-center">
            <Button variant="secondary" onClick={() => navigate(-1)}>Go back</Button>
          </div>
        </div>
      </SceneShell>
    )
  }

  const state: ReplayState = reconstructAt(replay.initial_hands, replay.moves, index)

  const handlePlay = () => setPlaying((p) => !p)
  const handleStepBack = () => {
    setPlaying(false)
    setIndex((prev) => Math.max(-1, prev - 1))
  }
  const handleStepForward = () => {
    setPlaying(false)
    setIndex((prev) => Math.min(totalMoves - 1, prev + 1))
  }
  const handleScrub = (e: React.ChangeEvent<HTMLInputElement>) => {
    setPlaying(false)
    setIndex(Number(e.target.value))
  }
  const handleSpeedCycle = () => {
    const idx = SPEEDS.indexOf(speed)
    setSpeed(SPEEDS[(idx + 1) % SPEEDS.length])
  }

  const roomLabel = replay.room_name || replay.game_id.slice(0, 8)
  const playerName = (idx: number) => replay.players[idx]?.display_name ?? `Player ${idx + 1}`
  const currentPlayerName = state.currentPlayer >= 0 ? playerName(state.currentPlayer) : null

  const isAtEnd = index === totalMoves - 1

  return (
    <SceneShell
      title={`Replay — ${roomLabel}`}
      eyebrow="Game Replay"
      action={
        <Button variant="secondary" onClick={() => navigate(-1)}>
          ← Back
        </Button>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[1fr_280px]">
        {/* Board + controls */}
        <div className="grid gap-3">
          {/* Players row */}
          <div className="flex flex-wrap items-end justify-center gap-3">
            {replay.players.map((p) => {
              const isActive = state.currentPlayer === p.player_index && !isAtEnd
              const handCount = state.hands[p.player_index]?.length ?? 0
              const faceDownCount = state.faceDown[p.player_index]?.length ?? 0
              return (
                <div
                  key={p.player_index}
                  className={`flex flex-col items-center gap-1 rounded-spade-lg border px-3 py-2 text-xs transition ${
                    isActive
                      ? 'border-spade-gold/60 bg-spade-bg/60 shadow-[0_0_10px_rgba(212,175,55,0.3)]'
                      : 'border-spade-cream/10 bg-spade-bg/30'
                  }`}
                >
                  <div className="size-8 rounded-full bg-spade-cream/10 grid place-items-center text-[11px] font-bold text-spade-cream">
                    {initialsForName(p.display_name)}
                  </div>
                  <span className="max-w-[72px] truncate font-medium text-spade-cream">{p.display_name}</span>
                  <div className="flex gap-2 text-[10px] text-spade-gray-3">
                    <span title="Cards in hand">🃏 {handCount}</span>
                    <span title="Face-down cards">⬇ {faceDownCount}</span>
                  </div>
                  {p.is_winner && (
                    <span className="text-[9px] font-semibold text-spade-gold">Winner</span>
                  )}
                </div>
              )
            })}
          </div>

          {/* Board */}
          <div className="mx-auto w-full max-w-[820px]">
            <GameBoard rows={state.boardRows} />
          </div>

          {/* Turn indicator */}
          <div className="text-center text-xs text-spade-gray-2">
            {isAtEnd
              ? 'Game over'
              : index === -1
              ? 'Initial deal — no moves yet'
              : currentPlayerName
              ? `${currentPlayerName}'s turn`
              : null}
          </div>

          {/* Playback controls */}
          <div className="mx-auto flex w-full max-w-[820px] flex-col gap-2">
            <input
              type="range"
              min={-1}
              max={totalMoves - 1}
              value={index}
              onChange={handleScrub}
              className="w-full accent-spade-gold"
              aria-label="Move scrubber"
            />
            <div className="flex items-center justify-center gap-2">
              <Button variant="secondary" onClick={handleStepBack} disabled={index <= -1} aria-label="Step back">
                ‹
              </Button>
              <Button variant={playing ? 'ghost' : 'primary'} onClick={handlePlay} disabled={isAtEnd}>
                {playing ? '⏸ Pause' : '▶ Play'}
              </Button>
              <Button variant="secondary" onClick={handleStepForward} disabled={isAtEnd} aria-label="Step forward">
                ›
              </Button>
              <Button variant="secondary" onClick={handleSpeedCycle} aria-label="Playback speed">
                {speed}×
              </Button>
            </div>
            <p className="text-center font-mono text-[10px] text-spade-gray-3">
              {index === -1 ? 'Deal' : `Move ${index + 1} / ${totalMoves}`}
            </p>
          </div>
        </div>

        {/* Move list sidebar */}
        <div className="flex flex-col gap-2">
          <h3 className="text-xs font-semibold uppercase tracking-wide text-spade-gray-2">
            Move list
          </h3>
          <div
            ref={moveListRef}
            className="max-h-[480px] overflow-y-auto rounded-spade-lg border border-spade-cream/10 bg-spade-bg/50"
          >
            {/* Deal row */}
            <button
              type="button"
              data-active={index === -1 ? 'true' : 'false'}
              onClick={() => { setPlaying(false); setIndex(-1) }}
              className={`w-full px-3 py-1.5 text-left text-xs transition ${
                index === -1
                  ? 'bg-spade-gold/15 text-spade-gold'
                  : 'text-spade-gray-2 hover:bg-spade-cream/5'
              }`}
            >
              <span className="font-mono text-[10px] text-spade-gray-3 mr-2">0</span>
              Initial deal
            </button>

            {replay.moves.map((move, i) => {
              const isActive = index === i
              const name = playerName(move.player_index)
              const suit = move.suit
              const suitColors: Record<string, string> = {
                hearts: 'text-spade-red',
                diamonds: 'text-spade-red',
                spades: 'text-spade-cream',
                clubs: 'text-spade-cream',
              }
              const suitClass = suitColors[suit] ?? 'text-spade-cream'
              return (
                <button
                  key={i}
                  type="button"
                  data-active={isActive ? 'true' : 'false'}
                  onClick={() => { setPlaying(false); setIndex(i) }}
                  className={`w-full border-t border-spade-cream/5 px-3 py-1.5 text-left text-xs transition ${
                    isActive
                      ? 'bg-spade-gold/15 text-spade-gold'
                      : 'text-spade-gray-2 hover:bg-spade-cream/5'
                  }`}
                >
                  <span className="font-mono text-[10px] text-spade-gray-3 mr-2">{i + 1}</span>
                  <span className="mr-1 font-medium text-spade-cream">{name}</span>
                  <span className={`font-mono font-bold ${suitClass}`}>
                    {rankLabel(move.rank)}{replaySuitSymbol(move.suit)}
                  </span>
                  {move.type === 'face_down' && (
                    <span className="ml-1 text-[9px] text-spade-gray-3">face-down</span>
                  )}
                  {move.type === 'ace_close' && (
                    <span className="ml-1 text-[9px] text-spade-gold">closes ({move.ace_direction})</span>
                  )}
                </button>
              )
            })}
          </div>

          {/* Game summary */}
          <div className="mt-2 rounded-spade-lg border border-spade-cream/10 bg-spade-bg/40 p-3">
            <h4 className="mb-2 text-[10px] font-semibold uppercase tracking-wide text-spade-gray-3">
              Final standings
            </h4>
            {[...replay.players]
              .sort((a, b) => a.rank - b.rank)
              .map((p) => (
                <div key={p.player_index} className="flex items-center justify-between py-0.5 text-xs">
                  <span className="text-spade-cream">
                    {p.rank}. {p.display_name}
                    {p.is_winner && <span className="ml-1 text-spade-gold">★</span>}
                  </span>
                </div>
              ))}
          </div>

          {/* Share link */}
          <p className="text-center text-[10px] text-spade-gray-3">
            Shareable link:{' '}
            <Link
              to={`/replay/${gameId}`}
              className="text-spade-gold-light underline underline-offset-2"
            >
              /replay/{gameId?.slice(0, 8)}…
            </Link>
          </p>
        </div>
      </div>
    </SceneShell>
  )
}
