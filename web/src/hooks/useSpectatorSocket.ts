import { useEffect, useMemo, useRef, useState } from 'react'
import type { BoardRow, GameResult, RevealedPenaltyCard } from '../types'
import { buildBoardRows, type WireBoardRange } from './useGameSocket'
import { normalizeRank, wireSuitToSuit } from '../game/cards'

const WS_URL = import.meta.env.VITE_WS_URL || 'ws://localhost:8081'

export type SpectatorStatus = 'idle' | 'connecting' | 'open' | 'closed' | 'error'

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

type SpectatorMessage = SpectatorStateMessage | SpectatorGameOverMessage | SpectatorErrorMessage

// useSpectatorSocket opens a read-only spectator connection and exposes the
// redacted live state. It never sends moves — there is no send function.
export function useSpectatorSocket(roomId: string | undefined, token: string | null): SpectatorState {
  const [status, setStatus] = useState<SpectatorStatus>('idle')
  const [notFound, setNotFound] = useState(false)
  const [gameOver, setGameOver] = useState(false)
  const [boardRows, setBoardRows] = useState<BoardRow[]>(() => buildBoardRows({}))
  const [players, setPlayers] = useState<SpectatorPlayer[]>([])
  const [currentTurnName, setCurrentTurnName] = useState<string | null>(null)
  const [turnEndsAt, setTurnEndsAt] = useState<string | null>(null)
  const [results, setResults] = useState<GameResult[]>([])
  const [connectionAttempt, setConnectionAttempt] = useState(0)
  const socketRef = useRef<WebSocket | null>(null)

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

      if (message.type === 'game_over') {
        setGameOver(true)
        if (message.board) {
          setBoardRows(buildBoardRows(message.board, message.closed_suits ?? [], message.ace_close_method))
        }
        setResults(message.results.map(toSpectatorResult))
        return
      }

      if (message.type === 'error') {
        // The server rejects an unknown / not-yet-started room with an error.
        setNotFound(true)
      }
    }

    socket.onerror = () => setStatus('error')
    socket.onclose = () => setStatus((current) => (current === 'error' ? current : 'closed'))

    return () => {
      window.clearTimeout(connectingTimer)
      socket.close()
      if (socketRef.current === socket) socketRef.current = null
    }
  }, [roomId, token, connectionAttempt])

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
      reconnect,
    }),
    [effectiveStatus, notFound, gameOver, boardRows, players, currentTurnName, turnEndsAt, results, reconnect],
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
