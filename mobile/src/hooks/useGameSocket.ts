import { type Dispatch, type SetStateAction, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { AppState } from 'react-native'
import type { BoardRow, Card, CloseMethod, GameResult, Player, Toast } from '../types'
import { boardColumns, initialsForName, normalizeRank, sequenceRankValue, suits, suitToWireSuit, wireSuitToSuit } from '../game/cards'
import { audioManager, type Cue } from '../game/sound'
import { decodeJwtClaims } from '../auth/claims'
import { WS_URL } from '../config'

// Ported from web/src/hooks/useGameSocket.ts. The message protocol, board/hand
// rebuilding, and sound-cue derivation are identical. Native additions:
//  - automatic reconnect with capped exponential backoff (the web app only
//    reconnects manually), and
//  - reconnect when the app returns to the foreground (the OS suspends sockets
//    while backgrounded).
// Timer calls use the global setTimeout/clearTimeout (React Native globals)
// rather than window.*.

const MAX_VISIBLE_TOASTS = 3
const TOAST_TTL_MS = 4000
const EMOTE_TTL_MS = 4000

// Reconnect backoff: doubles each attempt up to a cap.
const RECONNECT_BASE_MS = 1000
const RECONNECT_MAX_MS = 15000

export type WireBoardRange = {
  low: number | string
  high: number | string
} | null

type StateUpdateMessage = {
  type: 'state_update'
  board: Record<string, WireBoardRange>
  closed_suits?: string[]
  ace_close_method?: string
  ace_close_options?: Array<{ suit: string; can_low: boolean; can_high: boolean }>
  your_hand: Array<{ suit: string; rank: string | number; valid?: boolean }>
  opponents?: Array<{ display_name: string; avatar_url?: string; hand_count: number; facedown_count: number; disconnected?: boolean }>
  current_turn: string
  turn_ends_at?: string
  practice_mode?: boolean
}

type GameOverMessage = {
  type: 'game_over'
  board?: Record<string, WireBoardRange>
  closed_suits?: string[]
  ace_close_method?: string
  practice_mode?: boolean
  results: Array<{
    display_name: string
    avatar_url?: string
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

type LobbyStateMessage = {
  type: 'lobby_state'
  host_display_name: string
  min_to_start: number
  max_players: number
  can_start: boolean
  practice_mode?: boolean
  players: Array<{
    display_name: string
    avatar_url?: string
    is_host: boolean
    ready: boolean
    disconnected: boolean
  }>
}

type EmoteMessage = {
  type: 'emote'
  display_name: string
  emote: string
}

type GameSocketMessage =
  | StateUpdateMessage
  | GameOverMessage
  | RematchStatusMessage
  | PlayerConnectionMessage
  | ErrorMessage
  | RematchCancelledMessage
  | LobbyStateMessage
  | EmoteMessage

export type GameSocketStatus = 'idle' | 'connecting' | 'open' | 'closed' | 'error'

export type LobbyPlayer = {
  displayName: string
  avatarUrl?: string
  isHost: boolean
  ready: boolean
  disconnected: boolean
}

export type LobbyState = {
  hostDisplayName: string
  minToStart: number
  maxPlayers: number
  canStart: boolean
  players: LobbyPlayer[]
}

export type ActiveEmote = {
  id: string
  seq: number
}

export type GameSocketState = {
  status: GameSocketStatus
  phase: 'lobby' | 'playing'
  lobby: LobbyState | null
  isHost: boolean
  iAmReady: boolean
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
  practiceMode: boolean
  emotes: Record<string, ActiveEmote>
  myDisplayName: string | null
  sendPlayCard: (card: Card, method?: CloseMethod) => void
  sendFaceDown: (card: Card) => void
  sendRematchVote: () => void
  sendSetReady: (ready: boolean) => void
  sendStartGame: () => void
  sendLeave: () => void
  sendEmote: (id: string) => void
  reconnect: () => void
}

function decodeJwtDisplayName(token: string | null): string | null {
  return decodeJwtClaims(token).displayName
}

export function useGameSocket(roomId: string | undefined, token: string | null): GameSocketState {
  const [status, setStatus] = useState<GameSocketStatus>('idle')
  const [phase, setPhase] = useState<'lobby' | 'playing'>('lobby')
  const [lobby, setLobby] = useState<LobbyState | null>(null)
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
  const [practiceMode, setPracticeMode] = useState(false)
  const [emotes, setEmotes] = useState<Record<string, ActiveEmote>>({})
  const [connectionAttempt, setConnectionAttempt] = useState(0)
  const socketRef = useRef<WebSocket | null>(null)
  const toastIdRef = useRef(0)
  const toastTimersRef = useRef<ReturnType<typeof setTimeout>[]>([])
  const emoteSeqRef = useRef(0)
  const emoteTimersRef = useRef<ReturnType<typeof setTimeout>[]>([])
  const soundStateRef = useRef<SoundState | null>(null)
  // Native reconnect bookkeeping: a backoff timer and the current retry count.
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const retryCountRef = useRef(0)
  // Set when we intentionally tear down (unmount / manual reconnect) so the
  // onclose handler doesn't schedule an auto-reconnect against a dead socket.
  const intentionalCloseRef = useRef(false)

  const pushToast = useCallback((toast: Omit<Toast, 'id'>) => {
    const id = ++toastIdRef.current
    setToasts((current) => [{ ...toast, id }, ...current].slice(0, MAX_VISIBLE_TOASTS))
    const timer = setTimeout(() => {
      setToasts((current) => current.filter((t) => t.id !== id))
    }, TOAST_TTL_MS)
    toastTimersRef.current.push(timer)
  }, [])

  useEffect(() => {
    const timers = toastTimersRef.current
    return () => {
      for (const t of timers) {
        clearTimeout(t)
      }
    }
  }, [])

  const showEmote = useCallback((displayName: string, id: string) => {
    const seq = ++emoteSeqRef.current
    setEmotes((current) => ({ ...current, [displayName]: { id, seq } }))
    const timer = setTimeout(() => {
      setEmotes((current) => {
        if (current[displayName]?.seq !== seq) return current
        const next = { ...current }
        delete next[displayName]
        return next
      })
    }, EMOTE_TTL_MS)
    emoteTimersRef.current.push(timer)
  }, [])

  useEffect(() => {
    const timers = emoteTimersRef.current
    return () => {
      for (const t of timers) {
        clearTimeout(t)
      }
    }
  }, [])

  const myDisplayName = useMemo(() => decodeJwtDisplayName(token), [token])
  const myAvatarUrl = useMemo(() => decodeJwtClaims(token).avatarUrl ?? undefined, [token])

  // bumpConnection forces the connect effect to re-run, used by both the manual
  // reconnect() action and the auto-reconnect backoff.
  const bumpConnection = useCallback(() => {
    setConnectionAttempt((current) => current + 1)
  }, [])

  useEffect(() => {
    if (!roomId || !token) {
      return undefined
    }

    intentionalCloseRef.current = false
    const connectingTimer = setTimeout(() => setStatus('connecting'), 0)
    const params = new URLSearchParams({ room_id: roomId, token })
    const socket = new WebSocket(WS_URL + '/ws?' + params.toString())
    socketRef.current = socket

    socket.onopen = () => {
      retryCountRef.current = 0
      setStatus('open')
    }

    socket.onmessage = (event: { data: string }) => {
      handleMessage(event.data, myDisplayName, myAvatarUrl, {
        setBoardRows,
        setHand,
        setPlayers,
        pushToast,
        setIsMyTurn,
        setCurrentTurnName,
        setTurnEndsAt,
        setRematchVotes,
        setRematchTotal,
        setGameOver,
        setResults,
        setPracticeMode,
        setLobby,
        setPhase,
        showEmote,
        playSound: (cue: Cue) => audioManager.play(cue),
        soundStateRef,
      })
    }

    socket.onerror = () => {
      setStatus('error')
    }

    socket.onclose = () => {
      setStatus((current) => (current === 'error' ? current : 'closed'))
      // Schedule an auto-reconnect unless we tore the socket down on purpose.
      if (!intentionalCloseRef.current) {
        const delay = Math.min(RECONNECT_BASE_MS * 2 ** retryCountRef.current, RECONNECT_MAX_MS)
        retryCountRef.current += 1
        reconnectTimerRef.current = setTimeout(() => {
          bumpConnection()
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
      if (socketRef.current === socket) {
        socketRef.current = null
      }
      setPhase('lobby')
      setLobby(null)
      setEmotes({})
      soundStateRef.current = null
    }
  }, [roomId, token, connectionAttempt, myDisplayName, myAvatarUrl, pushToast, showEmote, bumpConnection])

  // Reconnect when the app returns to the foreground if the socket has dropped
  // (the OS commonly suspends sockets while backgrounded).
  useEffect(() => {
    if (!roomId || !token) return undefined
    const subscription = AppState.addEventListener('change', (next) => {
      if (next === 'active') {
        const sock = socketRef.current
        if (!sock || sock.readyState === WebSocket.CLOSED || sock.readyState === WebSocket.CLOSING) {
          retryCountRef.current = 0
          bumpConnection()
        }
      }
    })
    return () => subscription.remove()
  }, [roomId, token, bumpConnection])

  const send = useCallback((payload: Record<string, unknown>) => {
    if (socketRef.current?.readyState !== WebSocket.OPEN) {
      pushToast({ tone: 'error', title: 'Connection closed', body: 'Reconnect before sending another move.' })
      return
    }
    socketRef.current.send(JSON.stringify(payload))
  }, [pushToast])

  const sendPlayCard = useCallback((card: Card, method?: CloseMethod) => {
    const payload: Record<string, unknown> = { type: 'play_card', suit: suitToWireSuit[card.suit], rank: card.rank }
    if (method) {
      payload.method = method
    }
    send(payload)
  }, [send])

  const sendFaceDown = useCallback((card: Card) => {
    send({ type: 'place_facedown', suit: suitToWireSuit[card.suit], rank: card.rank })
  }, [send])

  const sendRematchVote = useCallback(() => {
    send({ type: 'rematch_vote' })
  }, [send])

  const sendSetReady = useCallback((ready: boolean) => {
    send({ type: 'set_ready', ready })
  }, [send])

  const sendStartGame = useCallback(() => {
    send({ type: 'start_game' })
  }, [send])

  const sendLeave = useCallback(() => {
    send({ type: 'leave' })
  }, [send])

  const sendEmote = useCallback((id: string) => {
    send({ type: 'emote', emote: id })
  }, [send])

  const reconnect = useCallback(() => {
    retryCountRef.current = 0
    bumpConnection()
  }, [bumpConnection])

  const effectiveStatus = roomId && token ? status : 'idle'

  const isHost = Boolean(
    myDisplayName && lobby?.players.some((p) => p.isHost && p.displayName === myDisplayName),
  )
  const iAmReady = Boolean(
    myDisplayName && lobby?.players.some((p) => p.displayName === myDisplayName && p.ready),
  )

  return useMemo(() => ({
    status: effectiveStatus,
    phase,
    lobby,
    isHost,
    iAmReady,
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
    practiceMode,
    emotes,
    myDisplayName,
    sendPlayCard,
    sendFaceDown,
    sendRematchVote,
    sendSetReady,
    sendStartGame,
    sendLeave,
    sendEmote,
    reconnect,
  }), [
    effectiveStatus,
    phase,
    lobby,
    isHost,
    iAmReady,
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
    practiceMode,
    emotes,
    myDisplayName,
    sendPlayCard,
    sendFaceDown,
    sendRematchVote,
    sendSetReady,
    sendStartGame,
    sendLeave,
    sendEmote,
    reconnect,
  ])
}

function handleMessage(
  rawMessage: string,
  myDisplayName: string | null,
  myAvatarUrl: string | undefined,
  setters: {
    setBoardRows: (rows: BoardRow[]) => void
    setHand: (cards: Card[]) => void
    setPlayers: Dispatch<SetStateAction<Player[]>>
    pushToast: (toast: Omit<Toast, 'id'>) => void
    setIsMyTurn: (isMyTurn: boolean) => void
    setCurrentTurnName: (name: string | null) => void
    setTurnEndsAt: (turnEndsAt: string | null) => void
    setRematchVotes: (votes: number) => void
    setRematchTotal: (total: number) => void
    setGameOver: (gameOver: boolean) => void
    setResults: (results: GameResult[]) => void
    setPracticeMode: (practiceMode: boolean) => void
    setLobby: (lobby: LobbyState | null) => void
    setPhase: (phase: 'lobby' | 'playing') => void
    showEmote: (displayName: string, id: string) => void
    playSound: (cue: Cue) => void
    soundStateRef: { current: SoundState | null }
  },
) {
  let message: GameSocketMessage
  try {
    message = JSON.parse(rawMessage) as GameSocketMessage
  } catch {
    setters.pushToast({ tone: 'error', title: 'Invalid message', body: 'The game server sent an unreadable update.' })
    return
  }

  if (message.type === 'lobby_state') {
    setters.setLobby({
      hostDisplayName: message.host_display_name,
      minToStart: message.min_to_start,
      maxPlayers: message.max_players,
      canStart: message.can_start,
      players: message.players.map((p) => ({
        displayName: p.display_name,
        avatarUrl: p.avatar_url || undefined,
        isHost: p.is_host,
        ready: p.ready,
        disconnected: p.disconnected,
      })),
    })
    setters.setPracticeMode(Boolean(message.practice_mode))
    setters.setPhase('lobby')
    return
  }

  if (message.type === 'state_update') {
    setters.setPhase('playing')
    setters.setBoardRows(buildBoardRows(message.board, message.closed_suits ?? [], message.ace_close_method))
    setters.setHand(buildHand(message))
    setters.setPlayers(buildPlayers(message, myAvatarUrl))
    const isMyTurn = myDisplayName ? message.current_turn === myDisplayName : Boolean(message.your_hand.some((card) => card.valid))
    setters.setIsMyTurn(isMyTurn)
    setters.setCurrentTurnName(isMyTurn ? 'You' : message.current_turn)
    setters.setTurnEndsAt(message.turn_ends_at ?? null)
    setters.setPracticeMode(Boolean(message.practice_mode))
    setters.setGameOver(false)
    setters.setResults([])
    setters.setRematchVotes(0)
    setters.setRematchTotal(4)

    const next = summarizeForSound(message, isMyTurn)
    for (const cue of detectStateUpdateCues(setters.soundStateRef.current, next)) {
      setters.playSound(cue)
    }
    setters.soundStateRef.current = next
    return
  }

  if (message.type === 'game_over') {
    setters.setGameOver(true)
    setters.setPracticeMode(Boolean(message.practice_mode))
    if (message.board) {
      setters.setBoardRows(buildBoardRows(message.board, message.closed_suits ?? [], message.ace_close_method))
    }
    const results = message.results.map(toGameResult)
    setters.setResults(results)
    setters.setPlayers(message.results.map((result, index) => ({
      name: result.display_name,
      initials: initialsForName(result.display_name),
      avatarUrl: result.avatar_url || undefined,
      cardsLeft: 0,
      faceDownCount: (result.facedown_cards ?? []).length,
      tone: playerTone(index),
      winner: result.is_winner,
      votedRematch: false,
    })))

    if (myDisplayName) {
      const mine = message.results.find((r) => r.display_name === myDisplayName)
      if (mine) {
        setters.playSound(mine.is_winner ? 'win' : 'lose')
      }
    }
    setters.soundStateRef.current = null
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
    setters.pushToast({
      tone: disconnected ? 'warn' : 'success',
      title: disconnected ? 'Player disconnected' : 'Player reconnected',
      body: message.display_name,
    })
    return
  }

  if (message.type === 'rematch_cancelled') {
    setters.setRematchVotes(0)
    setters.setPlayers((current) => current.map((player) => ({ ...player, votedRematch: false })))
    setters.pushToast({ tone: 'warn', title: 'Rematch cancelled', body: 'A player left before all votes were in.' })
    return
  }

  if (message.type === 'error') {
    setters.pushToast({ tone: 'error', title: 'Game error', body: message.message })
    return
  }

  if (message.type === 'emote') {
    setters.showEmote(message.display_name, message.emote)
  }
}

export type SoundState = {
  boardCardCount: number
  closedSuitCount: number
  handCount: number
  isMyTurn: boolean
}

function summarizeForSound(message: StateUpdateMessage, isMyTurn: boolean): SoundState {
  let boardCardCount = 0
  for (const range of Object.values(message.board)) {
    if (!range) continue
    const low = sequenceRankValue(range.low)
    const high = sequenceRankValue(range.high)
    if (high >= low && low > 0) {
      boardCardCount += high - low + 1
    }
  }
  return {
    boardCardCount,
    closedSuitCount: (message.closed_suits ?? []).length,
    handCount: message.your_hand.length,
    isMyTurn,
  }
}

export function detectStateUpdateCues(prev: SoundState | null, next: SoundState): Cue[] {
  const cues: Cue[] = []
  if (prev) {
    const boardGrew =
      next.boardCardCount > prev.boardCardCount || next.closedSuitCount > prev.closedSuitCount
    if (boardGrew) {
      cues.push('card_play')
    } else if (next.handCount < prev.handCount) {
      cues.push('facedown')
    }
    if (next.isMyTurn && !prev.isMyTurn) {
      cues.push('your_turn')
    }
  } else if (next.isMyTurn) {
    cues.push('your_turn')
  }
  return cues
}

export function buildBoardRows(
  board: Record<string, WireBoardRange>,
  closedSuits: string[] = [],
  aceCloseMethod?: string,
): BoardRow[] {
  const method: CloseMethod | undefined =
    aceCloseMethod === 'low' || aceCloseMethod === 'high' ? aceCloseMethod : undefined

  return suits.map((suit) => {
    const wireSuit = suitToWireSuit[suit]
    const range = board[wireSuit]
    const closed = closedSuits.includes(wireSuit)
    const aceEnd = closed ? method : undefined

    const low = range ? sequenceRankValue(range.low) : 0
    const high = range ? sequenceRankValue(range.high) : 0

    const cards = boardColumns.map((rank, index) => {
      if (index === 0) {
        return aceEnd === 'low' ? 'A' : null
      }
      if (index === boardColumns.length - 1) {
        return aceEnd === 'high' ? 'A' : null
      }
      if (!range) {
        return null
      }
      const value = index + 1
      return value >= low && value <= high ? rank : null
    })

    return { suit, closed, aceEnd, cards }
  })
}

function buildHand(message: StateUpdateMessage): Card[] {
  const options = new Map(
    (message.ace_close_options ?? []).map((option) => [option.suit, option] as const),
  )
  return message.your_hand.map((card) => {
    const base = toCard(card)
    const option = options.get(card.suit)
    if (base.rank === 'A' && option) {
      base.aceClose = { canLow: option.can_low, canHigh: option.can_high }
    }
    return base
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

function buildPlayers(message: StateUpdateMessage, myAvatarUrl: string | undefined): Player[] {
  return [
    {
      name: 'You',
      initials: 'YU',
      avatarUrl: myAvatarUrl,
      cardsLeft: message.your_hand.length,
      faceDownCount: 0,
      tone: 'green',
      active: message.your_hand.some((card) => card.valid),
    },
    ...(message.opponents ?? []).map((opponent, index) => ({
      name: opponent.display_name,
      initials: initialsForName(opponent.display_name),
      avatarUrl: opponent.avatar_url || undefined,
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
