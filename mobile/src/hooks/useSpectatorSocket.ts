import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { AppState } from 'react-native'
import type { BoardRow, GameResult, RevealedPenaltyCard } from '../types'
import { buildBoardRows, type WireBoardRange } from './useGameSocket'
import { normalizeRank, wireSuitToSuit } from '../game/cards'
import { WS_URL } from '../config'

// Ported from web/src/hooks/useSpectatorSocket.ts. Spectators are read-only with
// respect to the game (never send moves) but may send cosmetic emotes. Native
// additions mirror useGameSocket: foreground reconnect on AppState change.
const RECONNECT_BASE_MS = 1000
const RECONNECT_MAX_MS = 15000

// Mirrors the server's spectatorEmoteCooldown (2s) and the spectator emote
// bubble TTL.
export const SPECTATOR_EMOTE_COOLDOWN_MS = 2000
const SPECTATOR_EMOTE_TTL_MS = 4000

export type SpectatorStatus = 'idle' | 'connecting' | 'open' | 'closed' | 'error'

export type SpectatorPlayer = {
  displayName: string
  avatarUrl?: string
  handCount: number
  faceDownCount: number
  disconnected: boolean
}

// SpectatorReaction is one live spectator emote bubble (including this client's
// own echoed emotes). seq makes each unique for keying and expiry.
export type SpectatorReaction = {
  seq: number
  spectatorId: string
  emote: string
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
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const retryCountRef = useRef(0)
  const intentionalCloseRef = useRef(false)
  const reactionSeqRef = useRef(0)
  const reactionTimersRef = useRef<Array<ReturnType<typeof setTimeout>>>([])
  const cooldownUntilRef = useRef(0)

  const showReaction = useCallback((spectatorId: string, emote: string) => {
    reactionSeqRef.current += 1
    const seq = reactionSeqRef.current
    setReactions((current) => [...current, { seq, spectatorId, emote }])
    const timer = setTimeout(() => {
      setReactions((current) => current.filter((r) => r.seq !== seq))
    }, SPECTATOR_EMOTE_TTL_MS)
    reactionTimersRef.current.push(timer)
  }, [])

  useEffect(() => {
    if (!roomId || !token) return undefined

    intentionalCloseRef.current = false
    const connectingTimer = setTimeout(() => setStatus('connecting'), 0)
    const params = new URLSearchParams({ room_id: roomId, token, role: 'spectator' })
    const socket = new WebSocket(WS_URL + '/ws?' + params.toString())
    socketRef.current = socket

    socket.onopen = () => {
      retryCountRef.current = 0
      setStatus('open')
    }

    socket.onmessage = (event: { data: string }) => {
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
        // Only fatal "not watchable" errors blank the view; a rejected emote
        // must not.
        if (message.message === 'room not found' || message.message === 'game has not started') {
          setNotFound(true)
        }
      }
    }

    socket.onerror = () => setStatus('error')
    socket.onclose = () => {
      setStatus((current) => (current === 'error' ? current : 'closed'))
      if (!intentionalCloseRef.current) {
        const delay = Math.min(RECONNECT_BASE_MS * 2 ** retryCountRef.current, RECONNECT_MAX_MS)
        retryCountRef.current += 1
        reconnectTimerRef.current = setTimeout(() => {
          setConnectionAttempt((n) => n + 1)
        }, delay)
      }
    }

    return () => {
      intentionalCloseRef.current = true
      clearTimeout(connectingTimer)
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current)
        reconnectTimerRef.current = null
      }
      socket.close()
      if (socketRef.current === socket) socketRef.current = null
    }
  }, [roomId, token, connectionAttempt, showReaction])

  // Clear pending reaction timers on unmount.
  useEffect(() => {
    const timers = reactionTimersRef.current
    return () => {
      timers.forEach((t) => clearTimeout(t))
    }
  }, [])

  const sendEmote = useCallback((emote: string) => {
    const socket = socketRef.current
    if (!socket || socket.readyState !== WebSocket.OPEN) return
    if (Date.now() < cooldownUntilRef.current) return
    socket.send(JSON.stringify({ type: 'emote', emote }))
    const until = Date.now() + SPECTATOR_EMOTE_COOLDOWN_MS
    cooldownUntilRef.current = until
    setEmoteCooldownUntil(until)
  }, [])

  useEffect(() => {
    if (!roomId || !token) return undefined
    const subscription = AppState.addEventListener('change', (next) => {
      if (next === 'active') {
        const sock = socketRef.current
        if (!sock || sock.readyState === WebSocket.CLOSED || sock.readyState === WebSocket.CLOSING) {
          retryCountRef.current = 0
          setConnectionAttempt((n) => n + 1)
        }
      }
    })
    return () => subscription.remove()
  }, [roomId, token])

  const reconnect = useMemo(
    () => () => {
      setNotFound(false)
      retryCountRef.current = 0
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
