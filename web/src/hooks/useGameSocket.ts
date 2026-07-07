import { type Dispatch, type SetStateAction, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { BoardRow, Card, CloseMethod, GameResult, Player, Toast } from '../types'
import { boardColumns, initialsForName, normalizeRank, sequenceRankValue, suits, suitToWireSuit, wireSuitToSuit } from '../game/cards'
import { audioManager, type Cue } from '../game/sound'
import {
  classifySpectatorReaction,
  newSpectatorReactionWindow,
  type SpectatorReactionWindow,
} from '../game/spectatorReactions'

const WS_URL = import.meta.env.VITE_WS_URL || 'ws://localhost:8081'

// Transient toast behaviour: keep only the most recent few, each auto-dismissing
// after a short delay, so notifications never pile up into a log.
const MAX_VISIBLE_TOASTS = 3
const TOAST_TTL_MS = 4000

// How long an emote bubble stays visible over a player's seat before it fades.
const EMOTE_TTL_MS = 4000

export type WireBoardRange = {
  low: number | string
  high: number | string
  stacks?: Record<string, number>
} | null

type StateUpdateMessage = {
  type: 'state_update'
  board: Record<string, WireBoardRange>
  closed_suits?: string[]
  ace_close_method?: string
  ace_close_options?: Array<{ suit: string; can_low: boolean; can_high: boolean }>
  your_hand: Array<{ suit: string; rank: string | number; valid?: boolean }>
  your_facedown?: Array<{ suit: string; rank: string | number }>
  your_facedown_count?: number
  opponents?: Array<{ display_name: string; avatar_url?: string; is_bot?: boolean; hand_count: number; facedown_count: number; disconnected?: boolean; team?: number; is_teammate?: boolean; hand?: Array<{ suit: string; rank: string | number }> }>
  current_turn: string
  turn_ends_at?: string
  turn_timer_seconds?: number
  practice_mode?: boolean
  team_info?: { team: number; team_penalty: number; teammates: string[] }
}

type GameOverMessage = {
  type: 'game_over'
  board?: Record<string, WireBoardRange>
  closed_suits?: string[]
  ace_close_method?: string
  practice_mode?: boolean
  team_mode?: string
  results: Array<{
    display_name: string
    avatar_url?: string
    penalty_points: number
    rank: number
    is_winner: boolean
    is_bot?: boolean
    team?: number
    facedown_cards?: Array<{ suit: string; rank: string | number; points: number }>
    rating_delta?: number
    rating_after?: number
    xp_delta?: number
    xp_after?: number
    level?: number
  }>
}

type RematchStatusMessage = {
  type: 'rematch_status'
  votes: number
  total: number
  players?: Array<{ display_name: string; voted: boolean; left?: boolean }>
}

type PlayerConnectionMessage = {
  type: 'player_disconnected' | 'player_reconnected'
  display_name: string
}

type ErrorMessage = {
  type: 'error'
  message: string
  fatal?: boolean
}

type RematchCancelledMessage = {
  type: 'rematch_cancelled'
}

type RematchCountdownMessage = {
  type: 'rematch_countdown'
  expires_at: string
}

type RoomClosedMessage = {
  type: 'room_closed'
  reason?: string
}

type LobbyStateMessage = {
  type: 'lobby_state'
  host_display_name: string
  min_to_start: number
  max_players: number
  can_start: boolean
  practice_mode?: boolean
  team_mode?: string
  players: Array<{
    display_name: string
    avatar_url?: string
    slot?: number
    is_host: boolean
    ready: boolean
    disconnected: boolean
    team?: number
  }>
}

type EmoteMessage = {
  type: 'emote'
  display_name: string
  emote: string
}

type SpectatorEmoteMessage = {
  type: 'spectator_emote'
  spectator_id: string
  emote: string
}

type GameSocketMessage =
  | StateUpdateMessage
  | GameOverMessage
  | RematchStatusMessage
  | PlayerConnectionMessage
  | ErrorMessage
  | RematchCancelledMessage
  | RematchCountdownMessage
  | RoomClosedMessage
  | LobbyStateMessage
  | EmoteMessage
  | SpectatorEmoteMessage

export type GameSocketStatus = 'idle' | 'connecting' | 'open' | 'closed' | 'error'

export type LobbyPlayer = {
  displayName: string
  avatarUrl?: string
  slot: number
  isHost: boolean
  ready: boolean
  disconnected: boolean
  team?: number
}

export type LobbyState = {
  hostDisplayName: string
  minToStart: number
  maxPlayers: number
  canStart: boolean
  teamMode?: string
  players: LobbyPlayer[]
}

// ActiveEmote is the most recent emote shown over a player's seat, keyed by
// display name in the emotes map. `seq` makes each arrival unique so repeating
// the same emote id re-triggers the bubble/animation.
export type ActiveEmote = {
  id: string
  seq: number
}

// PlayerSpectatorReaction is one spectator emote surfaced to a seated player.
// Players see only the first few per window individually (kind: 'individual');
// the rest collapse into a single aggregate entry whose count climbs as more
// arrive (kind: 'aggregate'), so a hyped crowd reads as "🎉 ×N" rather than a
// flood of bubbles.
export type PlayerSpectatorReaction =
  | { kind: 'individual'; seq: number; emote: string }
  | { kind: 'aggregate'; emote: string; count: number }

export type GameSocketState = {
  status: GameSocketStatus
  phase: 'lobby' | 'playing'
  lobby: LobbyState | null
  isHost: boolean
  iAmReady: boolean
  boardRows: BoardRow[]
  hand: Card[]
  myFaceDown: Card[]
  players: Player[]
  toasts: Toast[]
  isMyTurn: boolean
  currentTurnName: string | null
  turnEndsAt: string | null
  turnTimerSeconds: number
  rematchVotes: number
  rematchTotal: number
  rematchEndsAt: string | null
  roomClosed: boolean
  gameOver: boolean
  results: GameResult[]
  practiceMode: boolean
  teamInfo: { team: number; teamPenalty: number; teammates: string[] } | null
  emotes: Record<string, ActiveEmote>
  spectatorReactions: PlayerSpectatorReaction[]
  myDisplayName: string | null
  sendPlayCard: (card: Card, method?: CloseMethod) => void
  sendFaceDown: (card: Card) => void
  sendRematchVote: () => void
  sendGoToWaitingRoom: () => void
  sendSetReady: (ready: boolean) => void
  sendStartGame: () => void
  sendLeave: () => void
  sendKick: (slot: number) => void
  sendEmote: (id: string) => void
  sendSetTeam: (team: number) => void
  reconnect: () => void
}

function decodeJwtDisplayName(token: string | null): string | null {
  return decodeJwtClaims(token).displayName
}

// decodeJwtClaims pulls the display_name and avatar_url out of the JWT payload
// so the local "You" seat can render its own identity (the WS opponent payloads
// only carry other players' avatars).
function decodeJwtClaims(token: string | null): { displayName: string | null; avatarUrl: string | null } {
  if (!token) return { displayName: null, avatarUrl: null }
  const parts = token.split('.')
  if (parts.length < 2) return { displayName: null, avatarUrl: null }
  try {
    const payload = JSON.parse(
      atob(parts[1].replace(/-/g, '+').replace(/_/g, '/')),
    ) as { display_name?: string; avatar_url?: string }
    return { displayName: payload.display_name ?? null, avatarUrl: payload.avatar_url ?? null }
  } catch {
    return { displayName: null, avatarUrl: null }
  }
}

export function useGameSocket(roomId: string | undefined, token: string | null): GameSocketState {
  const [status, setStatus] = useState<GameSocketStatus>('idle')
  const [phase, setPhase] = useState<'lobby' | 'playing'>('lobby')
  const [lobby, setLobby] = useState<LobbyState | null>(null)
  const [boardRows, setBoardRows] = useState<BoardRow[]>(() => buildBoardRows({}))
  const [hand, setHand] = useState<Card[]>([])
  const [myFaceDown, setMyFaceDown] = useState<Card[]>([])
  const [players, setPlayers] = useState<Player[]>([])
  const [toasts, setToasts] = useState<Toast[]>([])
  const [isMyTurn, setIsMyTurn] = useState(false)
  const [currentTurnName, setCurrentTurnName] = useState<string | null>(null)
  const [turnEndsAt, setTurnEndsAt] = useState<string | null>(null)
  const [turnTimerSeconds, setTurnTimerSeconds] = useState(60)
  const [rematchVotes, setRematchVotes] = useState(0)
  const [rematchTotal, setRematchTotal] = useState(4)
  const [rematchEndsAt, setRematchEndsAt] = useState<string | null>(null)
  const [roomClosed, setRoomClosed] = useState(false)
  const [gameOver, setGameOver] = useState(false)
  const [results, setResults] = useState<GameResult[]>([])
  const [practiceMode, setPracticeMode] = useState(false)
  const [teamInfo, setTeamInfo] = useState<{ team: number; teamPenalty: number; teammates: string[] } | null>(null)
  const [emotes, setEmotes] = useState<Record<string, ActiveEmote>>({})
  const [spectatorReactions, setSpectatorReactions] = useState<PlayerSpectatorReaction[]>([])
  const [connectionAttempt, setConnectionAttempt] = useState(0)
  const socketRef = useRef<WebSocket | null>(null)
  const toastIdRef = useRef(0)
  const toastTimersRef = useRef<number[]>([])
  const emoteSeqRef = useRef(0)
  const emoteTimersRef = useRef<number[]>([])
  // Player-facing spectator-reaction throttle: the rolling window state and a
  // separate seq + timers for the individual reaction bubbles.
  const spectatorReactionWindowRef = useRef<SpectatorReactionWindow>(newSpectatorReactionWindow())
  const spectatorReactionSeqRef = useRef(0)
  const spectatorReactionTimersRef = useRef<number[]>([])
  const aggregateTimerRef = useRef<number | null>(null)
  // Tracks the prior state_update so the message handler can derive sound cues
  // (board grew -> card_play, hand-only shrink -> facedown, turn flipped to me
  // -> your_turn). Reset on (re)connect below.
  const soundStateRef = useRef<SoundState | null>(null)

  // pushToast adds a transient notification: it caps the visible stack to the
  // most recent few and auto-dismisses each one after a few seconds so toasts
  // don't accumulate into a log.
  const pushToast = useCallback((toast: Omit<Toast, 'id'>) => {
    const id = ++toastIdRef.current
    setToasts((current) => [{ ...toast, id }, ...current].slice(0, MAX_VISIBLE_TOASTS))
    const timer = window.setTimeout(() => {
      setToasts((current) => current.filter((t) => t.id !== id))
    }, TOAST_TTL_MS)
    toastTimersRef.current.push(timer)
  }, [])

  // Clear any pending dismiss timers on unmount.
  useEffect(() => {
    const timers = toastTimersRef.current
    return () => {
      for (const t of timers) {
        window.clearTimeout(t)
      }
    }
  }, [])

  // showEmote records the latest emote for a player (keyed by display name) and
  // schedules it to clear after a short TTL, so bubbles fade on their own.
  const showEmote = useCallback((displayName: string, id: string) => {
    const seq = ++emoteSeqRef.current
    setEmotes((current) => ({ ...current, [displayName]: { id, seq } }))
    const timer = window.setTimeout(() => {
      setEmotes((current) => {
        // Only clear if this is still the emote we scheduled (a newer one may
        // have replaced it).
        if (current[displayName]?.seq !== seq) return current
        const next = { ...current }
        delete next[displayName]
        return next
      })
    }, EMOTE_TTL_MS)
    emoteTimersRef.current.push(timer)
  }, [])

  // Clear any pending emote timers on unmount.
  useEffect(() => {
    const timers = emoteTimersRef.current
    return () => {
      for (const t of timers) {
        window.clearTimeout(t)
      }
    }
  }, [])

  // showSpectatorReaction surfaces a spectator emote to the seated player,
  // throttled: the first few per rolling window appear as their own bubbles
  // (auto-clearing after the TTL); the rest fold into a single aggregate
  // "<emote> ×N" entry that also clears once the window goes quiet.
  const showSpectatorReaction = useCallback((emote: string) => {
    const decision = classifySpectatorReaction(spectatorReactionWindowRef.current, Date.now())
    spectatorReactionWindowRef.current = decision.window

    if (decision.show === 'individual') {
      const seq = ++spectatorReactionSeqRef.current
      setSpectatorReactions((current) => [...current, { kind: 'individual', seq, emote }])
      const timer = window.setTimeout(() => {
        setSpectatorReactions((current) =>
          current.filter((r) => !(r.kind === 'individual' && r.seq === seq)),
        )
      }, EMOTE_TTL_MS)
      spectatorReactionTimersRef.current.push(timer)
      return
    }

    // Aggregate: replace any existing aggregate entry with the overflow count
    // (total minus individual limit), so it reads as "<emote> ×N".
    setSpectatorReactions((current) => {
      const withoutAggregate = current.filter((r) => r.kind !== 'aggregate')
      return [...withoutAggregate, { kind: 'aggregate', emote, count: decision.aggregateCount }]
    })
    if (aggregateTimerRef.current !== null) window.clearTimeout(aggregateTimerRef.current)
    aggregateTimerRef.current = window.setTimeout(() => {
      setSpectatorReactions((current) => current.filter((r) => r.kind !== 'aggregate'))
      aggregateTimerRef.current = null
    }, EMOTE_TTL_MS)
  }, [])

  // Clear any pending spectator-reaction timers on unmount.
  useEffect(() => {
    const timers = spectatorReactionTimersRef.current
    return () => {
      for (const t of timers) {
        window.clearTimeout(t)
      }
      if (aggregateTimerRef.current !== null) window.clearTimeout(aggregateTimerRef.current)
    }
  }, [])

  const myDisplayName = useMemo(() => decodeJwtDisplayName(token), [token])
  const myAvatarUrl = useMemo(() => decodeJwtClaims(token).avatarUrl ?? undefined, [token])

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
      handleMessage(event.data, myDisplayName, myAvatarUrl, {
        setBoardRows,
        setHand,
        setMyFaceDown,
        setPlayers,
        pushToast,
        setIsMyTurn,
        setCurrentTurnName,
        setTurnEndsAt,
        setTurnTimerSeconds,
        setRematchVotes,
        setRematchTotal,
        setRematchEndsAt,
        setRoomClosed,
        setGameOver,
        setResults,
        setPracticeMode,
        setTeamInfo,
        setLobby,
        setPhase,
        showEmote,
        showSpectatorReaction,
        playSound: (cue: Cue) => audioManager.play(cue),
        soundStateRef,
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
      // Reset phase so re-mount starts in lobby again.
      setPhase('lobby')
      setLobby(null)
      setEmotes({})
      setSpectatorReactions([])
      spectatorReactionWindowRef.current = newSpectatorReactionWindow()
      soundStateRef.current = null
    }
    // myDisplayName/myAvatarUrl are derived from token (memoised), so they only
    // change when token does — including them keeps the socket's onmessage
    // closure correct without causing extra reconnects. pushToast/showEmote are
    // stable useCallbacks.
  }, [roomId, token, connectionAttempt, myDisplayName, myAvatarUrl, pushToast, showEmote, showSpectatorReaction])

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

  const sendGoToWaitingRoom = useCallback(() => {
    send({ type: 'go_to_waiting_room' })
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

  const sendKick = useCallback((slot: number) => {
    send({ type: 'kick', target: slot })
  }, [send])

  const sendEmote = useCallback((id: string) => {
    send({ type: 'emote', emote: id })
  }, [send])

  const sendSetTeam = useCallback((team: number) => {
    send({ type: 'set_team', team })
  }, [send])

  const reconnect = useCallback(() => {
    setConnectionAttempt((current) => current + 1)
  }, [])

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
    myFaceDown,
    players,
    toasts,
    isMyTurn,
    currentTurnName,
    turnEndsAt,
    turnTimerSeconds,
    rematchVotes,
    rematchTotal,
    rematchEndsAt,
    roomClosed,
    gameOver,
    results,
    practiceMode,
    teamInfo,
    emotes,
    spectatorReactions,
    myDisplayName,
    sendPlayCard,
    sendFaceDown,
    sendRematchVote,
    sendGoToWaitingRoom,
    sendSetReady,
    sendStartGame,
    sendLeave,
    sendKick,
    sendEmote,
    sendSetTeam,
    reconnect,
  }), [
    effectiveStatus,
    phase,
    lobby,
    isHost,
    iAmReady,
    boardRows,
    hand,
    myFaceDown,
    players,
    toasts,
    isMyTurn,
    currentTurnName,
    turnEndsAt,
    turnTimerSeconds,
    rematchVotes,
    rematchTotal,
    rematchEndsAt,
    roomClosed,
    gameOver,
    results,
    practiceMode,
    teamInfo,
    emotes,
    spectatorReactions,
    myDisplayName,
    sendPlayCard,
    sendFaceDown,
    sendRematchVote,
    sendGoToWaitingRoom,
    sendSetReady,
    sendStartGame,
    sendLeave,
    sendKick,
    sendEmote,
    sendSetTeam,
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
    setMyFaceDown: (cards: Card[]) => void
    setPlayers: Dispatch<SetStateAction<Player[]>>
    pushToast: (toast: Omit<Toast, 'id'>) => void
    setIsMyTurn: (isMyTurn: boolean) => void
    setCurrentTurnName: (name: string | null) => void
    setTurnEndsAt: (turnEndsAt: string | null) => void
    setTurnTimerSeconds: (turnTimerSeconds: number) => void
    setRematchVotes: (votes: number) => void
    setRematchTotal: (total: number) => void
    setRematchEndsAt: (endsAt: string | null) => void
    setRoomClosed: (closed: boolean) => void
    setGameOver: (gameOver: boolean) => void
    setResults: (results: GameResult[]) => void
    setPracticeMode: (practiceMode: boolean) => void
    setTeamInfo: (teamInfo: { team: number; teamPenalty: number; teammates: string[] } | null) => void
    setLobby: (lobby: LobbyState | null) => void
    setPhase: (phase: 'lobby' | 'playing') => void
    showEmote: (displayName: string, id: string) => void
    showSpectatorReaction: (emote: string) => void
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
      teamMode: message.team_mode,
      players: message.players.map((p, index) => ({
        displayName: p.display_name,
        avatarUrl: p.avatar_url || undefined,
        slot: p.slot ?? index,
        isHost: p.is_host,
        ready: p.ready,
        disconnected: p.disconnected,
        team: p.team,
      })),
    })
    setters.setPracticeMode(Boolean(message.practice_mode))
    setters.setPhase('lobby')
    setters.setGameOver(false)
    setters.setRematchEndsAt(null)
    setters.setRematchVotes(0)
    return
  }

  if (message.type === 'state_update') {
    setters.setPhase('playing')
    setters.setBoardRows(buildBoardRows(message.board, message.closed_suits ?? [], message.ace_close_method))
    setters.setHand(buildHand(message))
    setters.setMyFaceDown((message.your_facedown ?? []).map(toCard))
    setters.setPlayers(buildPlayers(message, myAvatarUrl))
    const isMyTurn = myDisplayName ? message.current_turn === myDisplayName : Boolean(message.your_hand.some((card) => card.valid))
    setters.setIsMyTurn(isMyTurn)
    setters.setCurrentTurnName(isMyTurn ? 'You' : message.current_turn)
    setters.setTurnEndsAt(message.turn_ends_at ?? null)
    // The expiry timestamp drives the label; the configured duration drives the
    // progress bar for rooms that are not using the 60s default.
    setters.setTurnTimerSeconds(message.turn_timer_seconds ?? 60)
    setters.setPracticeMode(Boolean(message.practice_mode))
    setters.setTeamInfo(message.team_info ? {
      team: message.team_info.team,
      teamPenalty: message.team_info.team_penalty,
      teammates: message.team_info.teammates,
    } : null)
    setters.setGameOver(false)
    setters.setResults([])
    setters.setRematchVotes(0)
    setters.setRematchTotal(4)
    setters.setRematchEndsAt(null)

    // Derive sound cues by diffing against the previous state_update.
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
    setters.setTeamInfo(message.team_mode === '2v2' ? { team: 0, teamPenalty: 0, teammates: [] } : null)
    if (message.board) {
      setters.setBoardRows(buildBoardRows(message.board, message.closed_suits ?? [], message.ace_close_method))
    }
    const results = message.results.map(toGameResult)
    setters.setResults(results)
    // The rematch vote targets connected humans only (bots never vote), so seed
    // the total from the non-bot results. Without this the panel shows the
    // default 4 until the first rematch_status arrives (e.g. 0/4 instead of 0/2).
    setters.setRematchVotes(0)
    setters.setRematchTotal(Math.max(1, message.results.filter((result) => !result.is_bot).length))
    setters.setRematchEndsAt(null)
    setters.setPlayers(message.results.map((result, index) => ({
      name: result.display_name,
      initials: initialsForName(result.display_name),
      avatarUrl: result.avatar_url || undefined,
      cardsLeft: 0,
      faceDownCount: (result.facedown_cards ?? []).length,
      tone: playerTone(index),
      bot: Boolean(result.is_bot),
      winner: result.is_winner,
      votedRematch: false,
    })))

    // Win/lose cue from the local player's result. Falls back silently when the
    // local identity can't be matched (e.g. spectator-less guest edge cases).
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
    setters.setPlayers((current) => current.map((player) => {
      const vote = message.players?.find((entry) => entry.display_name === player.name)
      if (!vote) return player
      return { ...player, votedRematch: Boolean(vote.voted), disconnected: Boolean(vote.left) }
    }))
    return
  }

  if (message.type === 'rematch_countdown') {
    setters.setRematchEndsAt(message.expires_at || null)
    return
  }

  if (message.type === 'room_closed') {
    // Either the host kicked us, or a rematch window closed without us. Surface
    // a reason when the host kicked us; the page effect routes us to the lobby.
    if (message.reason === 'kicked') {
      setters.pushToast({ tone: 'warn', title: 'Removed from room', body: 'The host removed you from the room.' })
    }
    setters.setRoomClosed(true)
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
    if (message.fatal) {
      // A fatal error ends the connection (a rejected join: kicked, room full,
      // already started). Surface the reason and route the user back to the
      // lobby via the same roomClosed flag the pages already watch, instead of
      // stranding them on an empty waiting room.
      setters.pushToast({ tone: 'error', title: 'Cannot join room', body: message.message })
      setters.setRoomClosed(true)
      return
    }
    setters.pushToast({ tone: 'error', title: 'Game error', body: message.message })
    return
  }

  if (message.type === 'emote') {
    setters.showEmote(message.display_name, message.emote)
  }

  if (message.type === 'spectator_emote') {
    setters.showSpectatorReaction(message.emote)
  }
}

// SoundState is the minimal snapshot of a state_update needed to derive audio
// cues by diffing consecutive updates.
export type SoundState = {
  // Total number of cards across all board sequences (grows by 1 on a play).
  boardCardCount: number
  // Number of closed suits (grows by 1 when a suit is closed with an Ace).
  closedSuitCount: number
  // The viewer's own hand size (shrinks by 1 on a play or a face-down).
  handCount: number
  isMyTurn: boolean
}

// summarizeForSound reduces a state_update to the fields the cue detector needs.
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

// detectStateUpdateCues compares the previous and next sound snapshots and
// returns the cues to play. A board that grew — either a sequence extended
// (boardCardCount up) or a suit closed with an Ace (closedSuitCount up) — means
// a card was played (card_play). The viewer's hand shrinking without any board
// growth means a face-down penalty (facedown). The turn flipping to the viewer
// plays your_turn. The first update of a game (prev == null) is silent except
// for the opening your_turn, since there is nothing to diff against.
//
// The closed-suit signal matters because closing a suit with an Ace doesn't
// change the sequence low/high (the Ace sits in a separate column), so without
// it an Ace close would be misheard as a face-down penalty.
export function detectStateUpdateCues(prev: SoundState | null, next: SoundState): Cue[] {
  const cues: Cue[] = []
  if (prev) {
    const boardGrew =
      next.boardCardCount > prev.boardCardCount || next.closedSuitCount > prev.closedSuitCount
    if (boardGrew) {
      cues.push('card_play')
    } else if (next.handCount < prev.handCount) {
      // Hand shrank but nothing landed on the board -> a face-down penalty.
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
    // The closing Ace sits in the low (col 0) or high (col 13) slot based on the
    // global close method. Only show it once the suit is actually closed.
    const aceEnd = closed ? method : undefined

    // Compute fills by numeric value over the 14-slot layout. Slots 1..12 hold
    // 2..K (value v -> column v - 1). Slot 0 / 13 are the low/high Ace columns.
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
      const value = index + 1 // column 1 -> value 2, ..., column 12 -> value 13
      return value >= low && value <= high ? rank : null
    })

    const stacks = range && range.stacks ? range.stacks : undefined

    return { suit, closed, aceEnd, cards, stacks }
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
    bot: Boolean(result.is_bot),
    team: result.team,
    faceDownCards: (result.facedown_cards ?? []).map((card) => ({
      ...toCard(card),
      points: card.points,
    })),
    ratingDelta: result.rating_delta,
    ratingAfter: result.rating_after,
    xpDelta: result.xp_delta,
    xpAfter: result.xp_after,
    level: result.level,
  }
}

function buildPlayers(message: StateUpdateMessage, myAvatarUrl: string | undefined): Player[] {
  return [
    {
      name: 'You',
      initials: 'YU',
      avatarUrl: myAvatarUrl,
      cardsLeft: message.your_hand.length,
      faceDownCount: message.your_facedown_count ?? message.your_facedown?.length ?? 0,
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
      bot: Boolean(opponent.is_bot),
      disconnected: opponent.disconnected,
      isTeammate: opponent.is_teammate,
      teammateHand: opponent.hand?.map((c) => ({ suit: String(c.suit), rank: String(c.rank) })),
    })),
  ]
}

function playerTone(index: number): Player['tone'] {
  const tones: Array<Player['tone']> = ['green', 'gold', 'dark', 'red']
  return tones[index % tones.length]
}
