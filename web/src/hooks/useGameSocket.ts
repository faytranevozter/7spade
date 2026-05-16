import { type Dispatch, type SetStateAction, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { BoardRow, Card, GameResult, Player, Toast } from '../types'
import { initialsForName, normalizeRank, ranks, suits, suitToWireSuit, wireSuitToSuit } from '../game/cards'

const WS_URL = import.meta.env.VITE_WS_URL || 'ws://localhost:8081'

type WireBoardRange = {
  low: number | string
  high: number | string
} | null

type StateUpdateMessage = {
  type: 'state_update'
  board: Record<string, WireBoardRange>
  closed_suits?: string[]
  your_hand: Array<{ suit: string; rank: string | number; valid?: boolean }>
  opponents?: Array<{ display_name: string; hand_count: number; facedown_count: number; disconnected?: boolean }>
  current_turn: string
  turn_ends_at?: string
}

type GameOverMessage = {
  type: 'game_over'
  results: Array<{
    display_name: string
    penalty_points: number
    rank: number
    is_winner: boolean
    facedown_cards?: Array<{ suit: string; rank: string | number; points: number }>
  }>
}

type RematchStatusMessage = {
  type: 'rematch_status'
  votes: number
  total: number
  players?: Array<{ display_name: string; voted: boolean }>
}

type PlayerConnectionMessage = {
  type: 'player_disconnected' | 'player_reconnected'
  display_name: string
}

type ErrorMessage = {
  type: 'error'
  message: string
}

type RematchCancelledMessage = {
  type: 'rematch_cancelled'
}

type GameSocketMessage =
  | StateUpdateMessage
  | GameOverMessage
  | RematchStatusMessage
  | PlayerConnectionMessage
  | ErrorMessage
  | RematchCancelledMessage

export type GameSocketStatus = 'idle' | 'connecting' | 'open' | 'closed' | 'error'

export type GameSocketState = {
  status: GameSocketStatus
  boardRows: BoardRow[]
  hand: Card[]
  players: Player[]
  toasts: Toast[]
  isMyTurn: boolean
  currentTurnName: string | null
  turnEndsAt: string | null
  rematchVotes: number
  rematchTotal: number
  gameOver: boolean
  results: GameResult[]
  sendPlayCard: (card: Card) => void
  sendFaceDown: (card: Card) => void
  sendRematchVote: () => void
  reconnect: () => void
}

export function useGameSocket(roomId: string | undefined, token: string | null): GameSocketState {
  const [status, setStatus] = useState<GameSocketStatus>('idle')
  const [boardRows, setBoardRows] = useState<BoardRow[]>(() => buildBoardRows({}))
  const [hand, setHand] = useState<Card[]>([])
  const [players, setPlayers] = useState<Player[]>([])
  const [toasts, setToasts] = useState<Toast[]>([])
  const [isMyTurn, setIsMyTurn] = useState(false)
  const [currentTurnName, setCurrentTurnName] = useState<string | null>(null)
  const [turnEndsAt, setTurnEndsAt] = useState<string | null>(null)
  const [rematchVotes, setRematchVotes] = useState(0)
  const [rematchTotal, setRematchTotal] = useState(4)
  const [gameOver, setGameOver] = useState(false)
  const [results, setResults] = useState<GameResult[]>([])
  const [connectionAttempt, setConnectionAttempt] = useState(0)
  const socketRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    if (!roomId || !token) {
      return undefined
    }

    const connectingTimer = window.setTimeout(() => setStatus('connecting'), 0)
    const params = new URLSearchParams({ room_id: roomId, token })
    const socket = new WebSocket(`${WS_URL}/ws?${params.toString()}`)
    socketRef.current = socket

    socket.onopen = () => {
      setStatus('open')
    }

    socket.onmessage = (event: MessageEvent<string>) => {
      handleMessage(event.data, {
        setBoardRows,
        setHand,
        setPlayers,
        setToasts,
        setIsMyTurn,
        setCurrentTurnName,
        setTurnEndsAt,
        setRematchVotes,
        setRematchTotal,
        setGameOver,
        setResults,
      })
    }

    socket.onerror = () => {
      setStatus('error')
    }

    socket.onclose = () => {
      setStatus((current) => (current === 'error' ? current : 'closed'))
    }

    return () => {
      window.clearTimeout(connectingTimer)
      socket.close()
      if (socketRef.current === socket) {
        socketRef.current = null
      }
    }
  }, [roomId, token, connectionAttempt])

  const send = useCallback((payload: Record<string, unknown>) => {
    if (socketRef.current?.readyState !== WebSocket.OPEN) {
      setToasts((current) => [
        { tone: 'error', title: 'Connection closed', body: 'Reconnect before sending another move.' },
        ...current,
      ])
      return
    }

    socketRef.current.send(JSON.stringify(payload))
  }, [])

  const sendPlayCard = useCallback((card: Card) => {
    send({ type: 'play_card', suit: suitToWireSuit[card.suit], rank: card.rank })
  }, [send])

  const sendFaceDown = useCallback((card: Card) => {
    send({ type: 'place_facedown', suit: suitToWireSuit[card.suit], rank: card.rank })
  }, [send])

  const sendRematchVote = useCallback(() => {
    send({ type: 'rematch_vote' })
  }, [send])

  const reconnect = useCallback(() => {
    setConnectionAttempt((current) => current + 1)
  }, [])

  const effectiveStatus = roomId && token ? status : 'idle'

  return useMemo(() => ({
    status: effectiveStatus,
    boardRows,
    hand,
    players,
    toasts,
    isMyTurn,
    currentTurnName,
    turnEndsAt,
    rematchVotes,
    rematchTotal,
    gameOver,
    results,
    sendPlayCard,
    sendFaceDown,
    sendRematchVote,
    reconnect,
  }), [
    effectiveStatus,
    boardRows,
    hand,
    players,
    toasts,
    isMyTurn,
    currentTurnName,
    turnEndsAt,
    rematchVotes,
    rematchTotal,
    gameOver,
    results,
    sendPlayCard,
    sendFaceDown,
    sendRematchVote,
    reconnect,
  ])
}

function handleMessage(
  rawMessage: string,
  setters: {
    setBoardRows: (rows: BoardRow[]) => void
    setHand: (cards: Card[]) => void
    setPlayers: Dispatch<SetStateAction<Player[]>>
    setToasts: Dispatch<SetStateAction<Toast[]>>
    setIsMyTurn: (isMyTurn: boolean) => void
    setCurrentTurnName: (name: string | null) => void
    setTurnEndsAt: (turnEndsAt: string | null) => void
    setRematchVotes: (votes: number) => void
    setRematchTotal: (total: number) => void
    setGameOver: (gameOver: boolean) => void
    setResults: (results: GameResult[]) => void
  },
) {
  let message: GameSocketMessage
  try {
    message = JSON.parse(rawMessage) as GameSocketMessage
  } catch {
    setters.setToasts((current) => [
      { tone: 'error', title: 'Invalid message', body: 'The game server sent an unreadable update.' },
      ...current,
    ])
    return
  }

  if (message.type === 'state_update') {
    setters.setBoardRows(buildBoardRows(message.board, message.closed_suits ?? []))
    setters.setHand(message.your_hand.map(toCard))
    setters.setPlayers(buildPlayers(message))
    const isMyTurn = Boolean(message.your_hand.some((card) => card.valid))
    setters.setIsMyTurn(isMyTurn)
    setters.setCurrentTurnName(isMyTurn ? 'You' : message.current_turn)
    setters.setTurnEndsAt(message.turn_ends_at ?? null)
    setters.setGameOver(false)
    setters.setResults([])
    setters.setRematchVotes(0)
    setters.setRematchTotal(4)
    return
  }

  if (message.type === 'game_over') {
    setters.setGameOver(true)
    const results = message.results.map(toGameResult)
    setters.setResults(results)
    setters.setPlayers(results.map((result, index) => ({
      name: result.player,
      initials: initialsForName(result.player),
      cardsLeft: 0,
      faceDownCount: result.faceDownCards.length,
      tone: playerTone(index),
      winner: result.winner,
      votedRematch: false,
    })))
    return
  }

  if (message.type === 'rematch_status') {
    setters.setRematchVotes(message.votes)
    setters.setRematchTotal(message.total)
    setters.setPlayers((current) => current.map((player) => ({
      ...player,
      votedRematch: Boolean(message.players?.some((vote) => vote.display_name === player.name && vote.voted)),
    })))
    return
  }

  if (message.type === 'player_disconnected' || message.type === 'player_reconnected') {
    const disconnected = message.type === 'player_disconnected'
    setters.setPlayers((current) => current.map((player) => (
      player.name === message.display_name ? { ...player, disconnected } : player
    )))
    setters.setToasts((current) => [
      {
        tone: disconnected ? 'warn' : 'success',
        title: disconnected ? 'Player disconnected' : 'Player reconnected',
        body: message.display_name,
      },
      ...current,
    ])
    return
  }

  if (message.type === 'rematch_cancelled') {
    setters.setRematchVotes(0)
    setters.setPlayers((current) => current.map((player) => ({ ...player, votedRematch: false })))
    setters.setToasts((current) => [
      { tone: 'warn', title: 'Rematch cancelled', body: 'A player left before all votes were in.' },
      ...current,
    ])
    return
  }

  if (message.type === 'error') {
    setters.setToasts((current) => [
      { tone: 'error', title: 'Game error', body: message.message },
      ...current,
    ])
  }
}

function buildBoardRows(board: Record<string, WireBoardRange>, closedSuits: string[] = []): BoardRow[] {
  return suits.map((suit) => {
    const wireSuit = suitToWireSuit[suit]
    const range = board[wireSuit]

    return {
      suit,
      closed: closedSuits.includes(wireSuit),
      cards: ranks.map((rank) => {
        if (!range) {
          return null
        }

        const lowIndex = ranks.indexOf(normalizeRank(range.low))
        const highIndex = ranks.indexOf(normalizeRank(range.high))
        const rankIndex = ranks.indexOf(rank)
        return rankIndex >= lowIndex && rankIndex <= highIndex ? rank : null
      }),
    }
  })
}

function toCard(card: { suit: string; rank: string | number; valid?: boolean }): Card {
  return {
    suit: wireSuitToSuit[card.suit] ?? 'Spades',
    rank: normalizeRank(card.rank),
    playable: Boolean(card.valid),
  }
}

function toGameResult(result: GameOverMessage['results'][number]): GameResult {
  return {
    player: result.display_name,
    rank: result.rank,
    penalty: result.penalty_points,
    winner: result.is_winner,
    faceDownCards: (result.facedown_cards ?? []).map((card) => ({
      ...toCard(card),
      points: card.points,
    })),
  }
}

function buildPlayers(message: StateUpdateMessage): Player[] {
  return [
    {
      name: 'You',
      initials: 'YU',
      cardsLeft: message.your_hand.length,
      faceDownCount: 0,
      tone: 'green',
      active: message.your_hand.some((card) => card.valid),
    },
    ...(message.opponents ?? []).map((opponent, index) => ({
      name: opponent.display_name,
      initials: initialsForName(opponent.display_name),
      cardsLeft: opponent.hand_count,
      faceDownCount: opponent.facedown_count,
      tone: playerTone(index + 1),
      active: opponent.display_name === message.current_turn,
      disconnected: opponent.disconnected,
    })),
  ]
}

function playerTone(index: number): Player['tone'] {
  const tones: Array<Player['tone']> = ['green', 'gold', 'dark', 'red']
  return tones[index % tones.length]
}
