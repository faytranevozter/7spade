import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { BoardRow, GameResult, RevealedPenaltyCard } from '../types'
import { buildBoardRows, type WireBoardRange } from './useGameSocket'
import { normalizeRank, wireSuitToSuit } from '../game/cards'

const WS_URL = import.meta.env.VITE_WS_URL || 'ws://localhost:8081'

// SPECTATOR_EMOTE_COOLDOWN_MS mirrors the server's spectatorEmoteCooldown (2s).
// The client disables the picker for this window so a spectator can't fire
// emotes the server will only silently drop, and so the picker can show a
// countdown.
export const SPECTATOR_EMOTE_COOLDOWN_MS = 2000

// SPECTATOR_EMOTE_TTL_MS is how long an incoming spectator reaction bubble stays
// on screen before it's cleared.
const SPECTATOR_EMOTE_TTL_MS = 4000

export type SpectatorStatus = 'idle' | 'connecting' | 'open' | 'closed' | 'error'

// SpectatorReaction is one live emote bubble from a spectator (including this
// client's own echoed emotes). seq makes each reaction unique so React can key
// and animate it, and so its expiry timer only clears the intended one.
export type SpectatorReaction = {
  seq: number
  spectatorId: string
  emote: string
}

// SpectatorPlayer is the public, redacted view of a seated player — no hand
// cards, only counts.
export type SpectatorPlayer = {
  displayName: string
  avatarUrl?: string
  handCount: number
  faceDownCount: number
  disconnected: boolean
}

export type SpectatorState = {
  status: SpectatorStatus
  notFound: boolean
  gameOver: boolean
  boardRows: BoardRow[]
  players: SpectatorPlayer[]
  currentTurnName: string | null
  turnEndsAt: string | null
  results: GameResult[]
  reactions: SpectatorReaction[]
  sendEmote: (emote: string) => void
  emoteCooldownUntil: number
  reconnect: () => void
}

type SpectatorStateMessage = {
  type: 'spectator_state'
  board: Record<string, WireBoardRange>
  closed_suits?: string[]
  ace_close_method?: string
  players: Array<{
    display_name: string
    avatar_url?: string
    hand_count: number
    facedown_count: number
    disconnected?: boolean
  }>
  current_turn: string
  turn_ends_at?: string
}

type SpectatorGameOverMessage = {
  type: 'game_over'
  board?: Record<string, WireBoardRange>
  closed_suits?: string[]
  ace_close_method?: string
  results: Array<{
    display_name: string
    avatar_url?: string
    penalty_points: number
    rank: number
    is_winner: boolean
    facedown_cards?: Array<{ suit: string; rank: string | number; points: number }>
  }>
}

type SpectatorErrorMessage = { type: 'error'; message: string }

type SpectatorEmoteMessage = {
  type: 'spectator_emote'
  spectator_id: string
  emote: string
}

type SpectatorMessage =
  | SpectatorStateMessage
  | SpectatorGameOverMessage
  | SpectatorErrorMessage
  | SpectatorEmoteMessage

// useSpectatorSocket opens a spectator connection and exposes the redacted live
// state. Spectators are read-only with respect to the game — they never send
// moves — but may send cosmetic emotes (sendEmote), which the server rebroadcasts
// to everyone as spectator_emote events surfaced here via reactions.
export function useSpectatorSocket(roomId: string | undefined, token: string | null): SpectatorState {
  const [status, setStatus] = useState<SpectatorStatus>('idle')
  const [notFound, setNotFound] = useState(false)
  const [gameOver, setGameOver] = useState(false)
  const [boardRows, setBoardRows] = useState<BoardRow[]>(() => buildBoardRows({}))
  const [players, setPlayers] = useState<SpectatorPlayer[]>([])
  const [currentTurnName, setCurrentTurnName] = useState<string | null>(null)
  const [turnEndsAt, setTurnEndsAt] = useState<string | null>(null)
  const [results, setResults] = useState<GameResult[]>([])
  const [reactions, setReactions] = useState<SpectatorReaction[]>([])
  const [emoteCooldownUntil, setEmoteCooldownUntil] = useState(0)
  const [connectionAttempt, setConnectionAttempt] = useState(0)
  const socketRef = useRef<WebSocket | null>(null)
  const reactionSeqRef = useRef(0)
  const reactionTimersRef = useRef<number[]>([])
  const cooldownUntilRef = useRef(0)

  // showReaction adds an incoming spectator emote bubble and schedules its
  // removal after the TTL (mirrors the player emote bubble lifecycle).
  const showReaction = useCallback((spectatorId: string, emote: string) => {
    reactionSeqRef.current += 1
    const seq = reactionSeqRef.current
    setReactions((current) => [...current, { seq, spectatorId, emote }])
    const timer = window.setTimeout(() => {
      setReactions((current) => current.filter((r) => r.seq !== seq))
    }, SPECTATOR_EMOTE_TTL_MS)
    reactionTimersRef.current.push(timer)
  }, [])

  useEffect(() => {
    if (!roomId || !token) return undefined

    const connectingTimer = window.setTimeout(() => setStatus('connecting'), 0)
    const params = new URLSearchParams({ room_id: roomId, token, role: 'spectator' })
    const socket = new WebSocket(`${WS_URL}/ws?${params.toString()}`)
    socketRef.current = socket

    socket.onopen = () => setStatus('open')

    socket.onmessage = (event: MessageEvent<string>) => {
      let message: SpectatorMessage
      try {
        message = JSON.parse(event.data) as SpectatorMessage
      } catch {
        return
      }

      if (message.type === 'spectator_state') {
        setGameOver(false)
        setBoardRows(buildBoardRows(message.board, message.closed_suits ?? [], message.ace_close_method))
        setPlayers(
          message.players.map((p) => ({
            displayName: p.display_name,
            avatarUrl: p.avatar_url || undefined,
            handCount: p.hand_count,
            faceDownCount: p.facedown_count,
            disconnected: Boolean(p.disconnected),
          })),
        )
        setCurrentTurnName(message.current_turn)
        setTurnEndsAt(message.turn_ends_at ?? null)
        return
      }

      if (message.type === 'spectator_emote') {
        showReaction(message.spectator_id, message.emote)
        return
      }

      if (message.type === 'game_over') {
        setGameOver(true)
        if (message.board) {
          setBoardRows(buildBoardRows(message.board, message.closed_suits ?? [], message.ace_close_method))
        }
        setResults(message.results.map(toSpectatorResult))
        return
      }

      if (message.type === 'error') {
        // "room not found" / "game has not started" mean the room isn't
        // watchable — surface the not-available view. Other errors (e.g. a
        // rejected emote) are non-fatal and must not blank the live view.
        if (message.message === 'room not found' || message.message === 'game has not started') {
          setNotFound(true)
        }
      }
    }

    // Guard against stale-socket callbacks: when this socket is superseded by a
    // newer one (reconnect / room change), its later async onerror/onclose must
    // not overwrite the active socket's status.
    socket.onerror = () => {
      if (socketRef.current === socket) setStatus('error')
    }
    socket.onclose = () => {
      if (socketRef.current === socket) {
        setStatus((current) => (current === 'error' ? current : 'closed'))
      }
    }

    return () => {
      window.clearTimeout(connectingTimer)
      // Detach handlers so this (now superseded) socket can't fire into state.
      socket.onopen = null
      socket.onmessage = null
      socket.onerror = null
      socket.onclose = null
      // Closing while CONNECTING triggers Chrome's "WebSocket is closed before
      // the connection is established" console error (common under StrictMode
      // double-mount). Wait for the handshake, then close.
      if (socket.readyState === WebSocket.CONNECTING) {
        socket.addEventListener('open', () => socket.close())
      } else if (socket.readyState === WebSocket.OPEN) {
        socket.close()
      }
      if (socketRef.current === socket) socketRef.current = null
    }
  }, [roomId, token, connectionAttempt, showReaction])

  // Clear any pending reaction-expiry timers on unmount.
  useEffect(() => {
    const timers = reactionTimersRef.current
    return () => {
      timers.forEach((t) => window.clearTimeout(t))
    }
  }, [])

  const sendEmote = useCallback((emote: string) => {
    const socket = socketRef.current
    if (!socket || socket.readyState !== WebSocket.OPEN) return
    // Client-side cooldown mirrors the server's 2s limit so we don't fire
    // emotes the server will only drop, and so the picker can show a countdown.
    if (Date.now() < cooldownUntilRef.current) return
    socket.send(JSON.stringify({ type: 'emote', emote }))
    const until = Date.now() + SPECTATOR_EMOTE_COOLDOWN_MS
    cooldownUntilRef.current = until
    setEmoteCooldownUntil(until)
  }, [])

  const reconnect = useMemo(
    () => () => {
      setNotFound(false)
      setConnectionAttempt((n) => n + 1)
    },
    [],
  )

  const effectiveStatus = roomId && token ? status : 'idle'

  return useMemo(
    () => ({
      status: effectiveStatus,
      notFound,
      gameOver,
      boardRows,
      players,
      currentTurnName,
      turnEndsAt,
      results,
      reactions,
      sendEmote,
      emoteCooldownUntil,
      reconnect,
    }),
    [
      effectiveStatus,
      notFound,
      gameOver,
      boardRows,
      players,
      currentTurnName,
      turnEndsAt,
      results,
      reactions,
      sendEmote,
      emoteCooldownUntil,
      reconnect,
    ],
  )
}

function toSpectatorResult(result: SpectatorGameOverMessage['results'][number]): GameResult {
  return {
    player: result.display_name,
    rank: result.rank,
    penalty: result.penalty_points,
    winner: result.is_winner,
    faceDownCards: (result.facedown_cards ?? []).map(
      (card): RevealedPenaltyCard => ({
        suit: wireSuitToSuit[card.suit] ?? 'Spades',
        rank: normalizeRank(card.rank),
        points: card.points,
      }),
    ),
  }
}
