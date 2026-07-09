import { createPortal } from 'react-dom'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { Avatar } from '../components/Avatar'
import { CardFace } from '../components/CardFace'
import { EmoteBubble } from '../components/EmoteBubble'
import { EmotePicker } from '../components/EmotePicker'
import { GameBoard } from '../components/GameBoard'
import { Modal } from '../components/Modal'
import { PiPBoard } from '../components/PiPBoard'
import { ScoreTable } from '../components/ScoreTable'
import { SectionPanel } from '../components/SectionPanel'
import { ToastStack } from '../components/ToastStack'
import { ApiError } from '../api/client'
import { getRoom } from '../api/lobby'
import { useAuth } from '../hooks/useAuth'
import { useGameSocket, type ActiveEmote, type GameSocketState, type PlayerSpectatorReaction } from '../hooks/useGameSocket'
import { useActiveRoom } from '../hooks/useActiveRoom'
import { usePiPContext } from '../hooks/PiPProvider'
import { useSound } from '../hooks/useSound'
import { emoteGlyph } from '../game/emotes'
import { wireSuitToSuit, suitSymbols } from '../game/cards'
import { getTeamColor } from '../game/teams'
import type { Card, GameResult, Player } from '../types'

// Matches the WS server's defaultRematchWindow. Drives the countdown progress
// bar on the results screen.
const REMATCH_WINDOW_SECONDS = 30

export function GamePage() {
  const { roomId } = useParams()
  const navigate = useNavigate()
  const { token, isAuthenticated } = useAuth()
  const game = useGameSocket(roomId, token)
  const { clear: clearActiveRoom, refresh: refreshActiveRoom } = useActiveRoom()
  const pip = usePiPContext()

  useEffect(() => {
    if (!pip.isOpen) return
    if (game.gameOver || game.roomClosed || game.status === 'closed' || game.status === 'error') {
      pip.closeWindow()
    }
  }, [pip, game.gameOver, game.roomClosed, game.status])

  const hasValidMoves = game.hand.some((card) => card.playable)
  const faceDownMode = game.isMyTurn && game.hand.length > 0 && !hasValidMoves
  const [closePrompt, setClosePrompt] = useState<Card | null>(null)
  const [selectedFaceDown, setSelectedFaceDown] = useState<Card | null>(null)

  useEffect(() => {
    if (!isAuthenticated) {
      navigate('/auth', { replace: true })
    }
  }, [isAuthenticated, navigate])

  // Verify the room exists and is still playable before joining. A 404 means it
  // never existed or was cleaned up — otherwise the WS server would silently
  // spin up a fresh empty room. A 'finished' room's live state may already be
  // gone from the game server's memory, so send the player to their history
  // (which holds the persisted results) rather than a phantom/empty board.
  useEffect(() => {
    if (!roomId || !token) return
    let cancelled = false
    getRoom(token, roomId)
      .then((room) => {
        if (cancelled) return
        if (room.status === 'finished') {
          navigate('/history', { replace: true })
        }
      })
      .catch((err: unknown) => {
        if (cancelled) return
        if (err instanceof ApiError && err.statusCode === 404) {
          navigate('/lobby', { replace: true })
        }
        // Other errors are transient; the socket/toast flow surfaces those.
      })
    return () => {
      cancelled = true
    }
  }, [roomId, token, navigate])

  // A partial rematch drops the voters back to the same room's waiting room: the
  // socket flips phase to 'lobby'. Only hand off once we've actually been in a
  // game this session — on first mount the socket starts in 'lobby' before the
  // first state_update, and we must not bounce a fresh join to the waiting room.
  const wasPlayingRef = useRef(false)
  useEffect(() => {
    if (game.phase === 'playing') {
      wasPlayingRef.current = true
      return
    }
    if (game.phase === 'lobby' && wasPlayingRef.current && roomId) {
      navigate(`/room/${roomId}`, { replace: true })
    }
  }, [game.phase, roomId, navigate])

  // The rematch window closed without us (we didn't vote, or nobody did and the
  // room was torn down). Head back to the main lobby.
  useEffect(() => {
    if (game.roomClosed) {
      navigate('/lobby', { replace: true })
    }
  }, [game.roomClosed, navigate])


  // passed). This adjust-state-during-render pattern is the React-recommended
  // alternative to an effect and prevents a stale, unconfirmed selection from
  // reappearing pre-selected when our turn comes back around.
  const [wasFaceDownMode, setWasFaceDownMode] = useState(faceDownMode)
  if (faceDownMode !== wasFaceDownMode) {
    setWasFaceDownMode(faceDownMode)
    if (!faceDownMode) {
      setSelectedFaceDown(null)
    }
  }

  // Only honour a selection while in face-down mode and the chosen card is still
  // in hand (guards against a same-turn state update removing it).
  const activeFaceDown = faceDownMode && selectedFaceDown
    && game.hand.some((card) => card.rank === selectedFaceDown.rank && card.suit === selectedFaceDown.suit)
    ? selectedFaceDown
    : null

  const visibleHand = useMemo(() => game.hand.map((card) => ({
    ...card,
    dimmed: game.isMyTurn && hasValidMoves && !card.playable,
    selected: faceDownMode
      ? activeFaceDown?.rank === card.rank && activeFaceDown?.suit === card.suit
      : card.selected,
  })), [game.hand, game.isMyTurn, hasValidMoves, faceDownMode, activeFaceDown])

  const playCard = (card: Card) => {
    if (!game.isMyTurn || !card.playable) {
      return
    }
    // First in-game interaction is a good moment to satisfy the autoplay policy.
    unlockSound()

    // An Ace play closes its suit. If both ends are legal and the global close
    // method isn't locked yet, ask the player which end to close; otherwise
    // resolve it directly (single end, or the server applies the locked method).
    if (card.aceClose) {
      const { canLow, canHigh } = card.aceClose
      if (canLow && canHigh) {
        setClosePrompt(card)
        return
      }
      game.sendPlayCard({ rank: card.rank, suit: card.suit, playable: card.playable }, canLow ? 'low' : 'high')
      return
    }

    game.sendPlayCard({ rank: card.rank, suit: card.suit, playable: card.playable })
  }

  const confirmClose = (method: 'low' | 'high') => {
    if (!closePrompt) return
    game.sendPlayCard({ rank: closePrompt.rank, suit: closePrompt.suit, playable: true }, method)
    setClosePrompt(null)
  }

  const selectFaceDown = (card: Card) => {
    setSelectedFaceDown(card)
  }

  const confirmFaceDown = () => {
    if (!activeFaceDown) return
    unlockSound()
    game.sendFaceDown({ rank: activeFaceDown.rank, suit: activeFaceDown.suit })
    setSelectedFaceDown(null)
  }

  const leaveRoom = () => {
    game.sendLeave()
    clearActiveRoom()
    refreshActiveRoom()
    navigate('/lobby')
  }

  const turnLabel = game.currentTurnName ? `${game.currentTurnName}'s turn` : 'Waiting...'
  const turnClock = useTurnClock(game.turnEndsAt, game.turnTimerSeconds)
  const { play: playSound, unlock: unlockSound } = useSound()
  const warnedTurnRef = useRef<string | null>(null)

  // Fire the timer-warning cue once when the local player's turn drops to ~5s.
  // Keyed by turnEndsAt so each turn warns at most once; the turn clock above
  // already re-renders every second to drive this check.
  useEffect(() => {
    if (!game.isMyTurn || !game.turnEndsAt) return
    const secondsLeft = Math.max(0, Math.ceil((Date.parse(game.turnEndsAt) - Date.now()) / 1000))
    if (secondsLeft <= 5 && secondsLeft > 0 && warnedTurnRef.current !== game.turnEndsAt) {
      warnedTurnRef.current = game.turnEndsAt
      playSound('timer_warning')
    }
  }, [game.isMyTurn, game.turnEndsAt, turnClock, playSound])

  if (game.gameOver) {
    return <GameOverPanel roomId={roomId} game={game} onLeave={leaveRoom} />
  }

  return (
    <div className="relative flex min-h-[calc(100svh-60px)] flex-col">
      {/* Main game table area */}
      <div className="relative flex flex-1 flex-col items-center justify-center gap-3 px-3 py-3 sm:px-4">
        {/* Opponents row */}
        <OpponentsRow players={game.players} currentTurnName={game.currentTurnName} emotes={game.emotes} />

        {/* Teammate hand strip */}
        <TeammateHandStrip players={game.players} />

        {/* Game board */}
        <div className="w-full max-w-[820px]">
          <GameBoard rows={game.boardRows} />

          {/* Turn timer bar */}
          {turnClock ? (
            <div className="mt-2 rounded-spade-pill border border-spade-cream/10 bg-spade-bg/70 p-1" aria-label="Turn countdown">
              <div
                aria-label="Turn time remaining"
                className="h-1.5 rounded-spade-pill bg-gradient-to-r from-spade-gold-light to-spade-gold transition-[width] duration-100 ease-linear"
                style={{ width: `${turnClock.percentRemaining}%` }}
              />
            </div>
          ) : null}

          {/* Turn indicator */}
          <div className="mt-2 flex items-center justify-center gap-3">
            {game.practiceMode ? <Badge tone="winner">Practice</Badge> : null}
            {game.teamInfo ? (
              <Badge tone="playing">
                {`Team ${game.teamInfo.team + 1} · ${game.teamInfo.teammates.length > 0 ? `with ${game.teamInfo.teammates.join(', ')}` : 'Solo'} · ${game.teamInfo.teamPenalty} pts`}
              </Badge>
            ) : null}
            <Badge tone={game.isMyTurn ? 'playing' : 'waiting'}>{game.isMyTurn ? '⚡ Your turn' : turnLabel}</Badge>
            {turnClock ? (
              <span role="timer" aria-label="Turn timer" className="rounded-spade-pill border border-spade-gold-light/40 bg-spade-gold/15 px-2.5 py-0.5 font-mono text-xs text-spade-gold-light">
                {turnClock.label}
              </span>
            ) : null}
          </div>
        </div>

        {/* Player hand */}
        <MyFaceDownPile cards={game.myFaceDown} />
        <div className="relative">
          {game.myDisplayName ? (
            <EmoteBubble
              emote={game.emotes[game.myDisplayName]}
              placementClassName="bottom-full left-2 mb-1"
            />
          ) : null}
          <PlayerHand
            cards={visibleHand}
            interactive={game.isMyTurn && hasValidMoves}
            onCardClick={playCard}
            isMyTurn={game.isMyTurn}
            faceDownMode={faceDownMode}
            onSelectFaceDown={selectFaceDown}
            onConfirmFaceDown={confirmFaceDown}
            hasFaceDownSelection={activeFaceDown !== null}
          />
        </div>
      </div>

      {/* Spectator reactions float along the bottom-center, distinct from the
          seat emote bubbles, so players feel the crowd without being spammed. */}
      <SpectatorReactionsOverlay reactions={game.spectatorReactions} />

      {/* Emote picker floats bottom-right, above the toast stack. */}
      <div className="fixed bottom-4 right-4 z-40">
        <EmotePicker onSelect={game.sendEmote} />
      </div>

      {/* Transient notifications float at the top-right, clear of the table and
          hand, and auto-dismiss (handled in useGameSocket). */}
      <div className="pointer-events-none fixed right-4 top-16 z-40 w-full max-w-xs">
        <ToastStack toasts={game.toasts} />
      </div>

      {closePrompt ? (
        <Modal
          title="Close the suit"
          eyebrow={`Ace of ${closePrompt.suit}`}
          description="This Ace can close the suit at either end. Your choice locks the closing method for every suit this round."
          onClose={() => setClosePrompt(null)}
          footer={
            <>
              <Button variant="secondary" onClick={() => setClosePrompt(null)}>
                Cancel
              </Button>
              <Button variant="secondary" onClick={() => confirmClose('high')}>
                Close high (Ace = 14)
              </Button>
              <Button onClick={() => confirmClose('low')}>
                Close low (Ace = 1)
              </Button>
            </>
          }
        >
          <p className="text-sm text-spade-gray-2">
            Closing low scores this Ace as 1 penalty point; closing high scores it as 14. The method applies to all suits closed this round.
          </p>
        </Modal>
      ) : null}

      {pip.isOpen && pip.container ? createPortal(
        <PiPBoard
          rows={game.boardRows}
          isMyTurn={game.isMyTurn}
          currentTurnName={game.currentTurnName}
          timerLabel={turnClock ? turnClock.label : null}
          timerPercent={turnClock ? turnClock.percentRemaining : null}
          hand={visibleHand}
          faceDownMode={faceDownMode}
          players={game.players}
          onPlayCard={game.sendPlayCard}
          onFaceDown={game.sendFaceDown}
        />,
        pip.container,
      ) : null}
    </div>
  )
}

// SpectatorReactionsOverlay shows spectator emotes to seated players, throttled
// by useGameSocket: the first few per window appear as individual bubbles and
// the rest collapse into a "<glyph> ×N" counter. It floats along the bottom
// center so it reads as crowd reaction, separate from the seat emote bubbles.
function SpectatorReactionsOverlay({ reactions }: { reactions: PlayerSpectatorReaction[] }) {
  if (reactions.length === 0) return null
  return (
    <div
      className="pointer-events-none fixed inset-x-0 bottom-20 z-30 flex flex-wrap items-center justify-center gap-1.5 px-4"
      role="status"
      aria-label="Spectator reactions"
    >
      {reactions.map((reaction) => {
        const glyph = emoteGlyph(reaction.emote)
        if (!glyph) return null
        const isWord = /[a-zA-Z]/.test(glyph)
        const glyphClass = isWord ? 'text-xs font-semibold' : 'text-base leading-none'
        if (reaction.kind === 'aggregate') {
          return (
            <span
              key="aggregate"
              className="flex items-center gap-1 rounded-spade-pill border border-spade-cream/20 bg-spade-cream/10 px-2 py-0.5 text-spade-cream"
            >
              <span className={glyphClass}>{glyph}</span>
              <span className="text-xs font-semibold">×{reaction.count}</span>
            </span>
          )
        }
        return (
          <span
            key={reaction.seq}
            className={`animate-emote-pop rounded-spade-pill border border-spade-cream/20 bg-spade-cream/10 px-2 py-0.5 text-spade-cream ${glyphClass}`}
          >
            {glyph}
          </span>
        )
      })}
    </div>
  )
}

function OpponentsRow({ players, currentTurnName, emotes }: { players: Player[]; currentTurnName: string | null; emotes: Record<string, ActiveEmote> }) {
  const opponents = players.filter((p) => p.name !== 'You')
  if (opponents.length === 0) return null

  return (
    <div className="flex w-full max-w-[820px] items-end justify-center gap-4 sm:gap-6">
      {opponents.map((player) => (
        <OpponentCard key={player.name} player={player} isCurrentTurn={player.name === currentTurnName} emote={emotes[player.name]} />
      ))}
    </div>
  )
}

function OpponentCard({ player, isCurrentTurn, emote }: { player: Player; isCurrentTurn: boolean; emote: ActiveEmote | undefined }) {
  const ringClass = isCurrentTurn ? 'ring-2 ring-spade-gold shadow-[0_0_12px_rgba(212,175,55,0.4)]' : ''
  const opacityClass = player.disconnected ? 'opacity-50' : ''
  const teammateClass = player.isTeammate ? 'border-spade-gold/40' : 'border-spade-cream/10'

  return (
    <div className={`relative flex flex-col items-center gap-1.5 rounded-spade-lg border bg-spade-bg/50 px-3 py-2 transition ${teammateClass} ${ringClass} ${opacityClass}`}>
      <EmoteBubble emote={emote} />
      <Avatar avatarUrl={player.avatarUrl} initials={player.initials} tone={player.tone} sizeClass="size-9" className="text-xs" />
      <span className="max-w-[80px] truncate text-xs font-medium text-spade-cream">{player.name}</span>
      {player.isTeammate ? <span className="text-[9px] font-medium text-spade-gold">Teammate</span> : null}
      <div className="flex items-center gap-2 text-[10px] text-spade-gray-3">
        <span
          key={`cards-${player.cardsLeft}`}
          className="anim-opponent-card-in"
          title="Cards in hand"
        >
          🃏 {player.cardsLeft}
        </span>
        <span title="Face-down cards">⬇ {player.faceDownCount}</span>
      </div>
      {player.disconnected ? <span className="text-[9px] text-red-400">Disconnected</span> : null}
    </div>
  )
}

function TeammateHandStrip({ players }: { players: Player[] }) {
  const teammates = players.filter((p) => p.isTeammate && p.teammateHand && p.teammateHand.length > 0)
  if (teammates.length === 0) return null

  return (
    <div className="w-full max-w-[820px]">
      {teammates.map((teammate) => (
        <div key={teammate.name} className="rounded-spade-md border border-blue-400/20 bg-blue-500/5 px-3 py-2">
          <div className="mb-1.5 flex items-center gap-2">
            <span className="inline-flex items-center gap-1.5 text-[10px] font-medium text-blue-300 before:block before:size-1.5 before:rounded-full before:bg-blue-400">
              {teammate.name}&apos;s hand
            </span>
            <span className="font-mono text-[9px] text-spade-gray-3">{teammate.teammateHand!.length} cards</span>
          </div>
          <div className="flex gap-0.5 overflow-x-auto pb-0.5">
            {teammate.teammateHand!.map((card, i) => {
              const suit = wireSuitToSuit[card.suit] ?? 'Spades'
              const symbol = suitSymbols[suit]
              const colorClass = card.suit === 'hearts' || card.suit === 'diamonds' ? 'text-[#e05c4a]' : 'text-[#d0cfc9]'
              return (
                <span
                  key={`${card.suit}-${card.rank}-${i}`}
                  className={`flex shrink-0 items-center gap-0.5 rounded border border-white/10 bg-white/5 px-1 py-0.5 text-[9px] font-bold ${colorClass}`}
                  title={`${card.rank} of ${card.suit}`}
                >
                  {card.rank}<span className="text-[8px]">{symbol}</span>
                </span>
              )
            })}
          </div>
        </div>
      ))}
    </div>
  )
}

// MyFaceDownPile renders a clickable badge showing the count of the player's
// own face-down penalty cards. Clicking it expands a small read-only card strip
// so the player can review what they've been forced to discard this round.
function MyFaceDownPile({ cards }: { cards: Card[] }) {
  const [expanded, setExpanded] = useState(false)
  if (!cards || cards.length === 0) return null
  return (
    <div className="flex flex-col items-center gap-2">
      <button
        type="button"
        onClick={() => setExpanded((prev) => !prev)}
        aria-expanded={expanded}
        aria-label={expanded ? 'Hide your face-down cards' : 'Show your face-down cards'}
        className="inline-flex items-center gap-1.5 rounded-spade-pill border border-spade-cream/15 bg-spade-bg/60 px-2.5 py-0.5 text-xs text-spade-gray-2 transition hover:border-spade-gold/40 hover:text-spade-cream"
      >
        <span>⬇ {cards.length} face-down</span>
        <span aria-hidden="true" className="text-[10px] text-spade-gray-3">{expanded ? '▲' : '▼'}</span>
      </button>
      {expanded ? (
        <div className="flex flex-wrap items-center justify-center gap-1 rounded-spade-lg border border-spade-cream/10 bg-spade-bg/50 px-2 py-2">
          {cards.map((card, i) => (
            <CardFace
              key={`${card.rank}-${card.suit}-${i}`}
              card={card}
              size="sm"
              interactive={false}
              ariaLabel={`Face-down ${card.rank} of ${card.suit}`}
            />
          ))}
        </div>
      ) : null}
    </div>
  )
}

function PlayerHand({
  cards,
  interactive,
  onCardClick,
  isMyTurn,
  faceDownMode,
  onSelectFaceDown,
  onConfirmFaceDown,
  hasFaceDownSelection,
}: {
  cards: Card[]
  interactive: boolean
  onCardClick: (card: Card) => void
  isMyTurn: boolean
  faceDownMode: boolean
  onSelectFaceDown: (card: Card) => void
  onConfirmFaceDown: () => void
  hasFaceDownSelection: boolean
}) {
  if (cards.length === 0) return null

  const totalCards = cards.length
  const compactThreshold = 13
  const maxRotation = totalCards > compactThreshold ? 0 : Math.min(totalCards * 2, 20)
  const overlap = totalCards > 20 ? 18 : totalCards > compactThreshold ? 12 : 5
  const maxWidth = totalCards > compactThreshold ? '100%' : '820px'
  const cardSize = totalCards > compactThreshold ? 'sm' as const : 'md' as const
  // In face-down mode every card is selectable; in normal play only highlighted
  // (playable) cards respond to clicks.
  const cardsInteractive = interactive || faceDownMode

  const handleClick = (card: Card) => {
    if (faceDownMode) {
      onSelectFaceDown(card)
      return
    }
    if (card.playable) {
      onCardClick(card)
    }
  }

  return (
    <div className="w-full" style={{ maxWidth }}>
      <div className="flex items-center justify-between px-1 pb-1">
        <span className="text-xs font-medium text-spade-cream/70">Your hand</span>
        <span className="font-mono text-[10px] text-spade-gray-3">{cards.length} cards</span>
      </div>
      <div className={`relative flex items-end justify-center pb-2 pt-4 ${totalCards > compactThreshold ? 'overflow-x-auto' : ''}`}>
        {cards.map((card, index) => {
          const centerOffset = index - (totalCards - 1) / 2
          const rotation = totalCards > compactThreshold ? 0 : (centerOffset / ((totalCards - 1) / 2 || 1)) * maxRotation
          const translateY = totalCards > compactThreshold ? 0 : Math.abs(centerOffset) * 2
          const clickable = faceDownMode || (interactive && card.playable)

          return (
            <div
              key={`${card.rank}-${card.suit}-${index}`}
              className="first:ml-0 transition-transform duration-150"
              style={{
                marginLeft: index === 0 ? 0 : `-${overlap}px`,
                transform: `rotate(${rotation}deg) translateY(${translateY}px)`,
                zIndex: index + 1,
              }}
            >
              <CardFace
                card={card}
                size={cardSize}
                interactive={cardsInteractive}
                onClick={clickable ? () => handleClick(card) : undefined}
                ariaLabel={faceDownMode ? `Select ${card.rank} of ${card.suit} for face down` : undefined}
                animationClassName={faceDownMode && card.selected ? 'anim-card-flip-down' : ''}
              />
            </div>
          )
        })}
      </div>
      {faceDownMode ? (
        <div className="flex flex-col items-center gap-2">
          <p className="text-center text-xs text-spade-gold/80">
            No valid moves — pick a card to place face down as a penalty.
          </p>
          <Button onClick={onConfirmFaceDown} disabled={!hasFaceDownSelection}>
            Place face-down
          </Button>
        </div>
      ) : isMyTurn ? (
        <p className="text-center text-xs text-spade-gold/80">
          Play a highlighted card to extend a suit
        </p>
      ) : null}
    </div>
  )
}

function GameOverPanel({
  roomId,
  game,
  onLeave,
}: {
  roomId: string | undefined
  game: GameSocketState
  onLeave: () => void
}) {
  const navigate = useNavigate()
  const hasSharedWin = game.results.filter((result) => result.winner).length > 1
  const winnerLabel = hasSharedWin ? 'Shared winner' : 'Winner'
  // The progress bar tracks the live countdown once voting opens; before the
  // first vote it sits full. The window is the server-side default (30s).
  const rematchClock = useTurnClock(game.rematchEndsAt, REMATCH_WINDOW_SECONDS)
  const countdownActive = Boolean(game.rematchEndsAt)
  const iVoted = Boolean(
    game.players.find((player) => player.name === game.myDisplayName)?.votedRematch,
  )
  // A rematch needs every human player present (bots don't count). If a human
  // left during the results screen, a full rematch is impossible — offer the
  // remaining players a move back to the waiting room instead. Practice games
  // are solo vs bots, so this never applies there.
  const someoneLeft = !game.practiceMode
    && game.players.some((player) => !player.bot && player.disconnected)
  const scores = game.results.map((result) => ({
    rank: result.rank,
    player: result.player,
    cardsLeft: 0,
    penalty: result.penalty,
    result: result.winner ? winnerLabel : 'Finished',
    winner: result.winner,
  }))

  return (
    <SectionPanel
      title="Results and rematch"
      eyebrow={game.practiceMode ? 'Practice Mode' : roomId ? `Room ${roomId}` : 'Game over + scoring'}
      action={<Badge tone="winner">{game.practiceMode ? 'Practice' : 'Round over'}</Badge>}
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_340px]">
        <div className="grid gap-4">
          <div className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
            <h3 className="text-lg font-medium">Final board</h3>
            <p className="mt-1 text-sm text-spade-gray-2">The completed sequences, including any suits closed with an Ace.</p>
            <div className="mt-4">
              <GameBoard rows={game.boardRows} />
            </div>
          </div>
          <ScoreTable scores={scores} winnerLabel={winnerLabel} />
          <MatchStatsCard results={game.results} myDisplayName={game.myDisplayName} practiceMode={game.practiceMode} />
          <RevealedPenaltyCards results={game.results} myDisplayName={game.myDisplayName} teamMode={game.teamInfo !== null} />
        </div>

        <div className="rounded-spade-lg border border-spade-gold/30 bg-spade-gold/10 p-4">
          <h3 className="text-lg font-medium">Rematch vote</h3>
          <p className="mt-1 text-sm text-spade-gray-2">
            {game.practiceMode
              ? 'Practice games are not saved to history or stats. Vote to play another round, or head back to the lobby.'
              : someoneLeft
                ? 'A player left, so a rematch with the full table is no longer possible. Head back to the waiting room to regroup or fill the seats with bots.'
                : countdownActive
                  ? 'When the countdown ends, everyone who voted heads back to the waiting room together. Players who did not vote leave the room.'
                  : 'Vote for a rematch to start a 30-second countdown. If everyone votes, the next game starts immediately.'}
          </p>
          <div className="mt-4 grid gap-2">
            {someoneLeft ? (
              <Button onClick={game.sendGoToWaitingRoom}>Go to waiting room</Button>
            ) : (
              <Button onClick={game.sendRematchVote} disabled={iVoted}>
                {iVoted ? 'Voted — waiting' : 'Vote rematch'}
              </Button>
            )}
            <Button variant="secondary" onClick={onLeave}>Leave room</Button>
            {game.practiceMode ? null : (
              <Button variant="ghost" onClick={() => navigate('/history')}>View history</Button>
            )}
          </div>
          {countdownActive && !someoneLeft ? (
            <>
              <div className="mt-4 h-2 overflow-hidden rounded-full bg-spade-bg/70" aria-label="Rematch countdown">
                <div
                  className="h-full rounded-full bg-spade-gold-light transition-[width] duration-100 ease-linear"
                  style={{ width: `${rematchClock?.percentRemaining ?? 0}%` }}
                />
              </div>
              <p className="mt-2 font-mono text-xs text-spade-gold-light">
                {`Rematch in ${rematchClock?.label ?? '00:00'} · ${game.rematchVotes} / ${game.rematchTotal} voted`}
              </p>
            </>
          ) : (
            <p className="mt-4 font-mono text-xs text-spade-gold-light">{game.rematchVotes} / {game.rematchTotal} voted</p>
          )}
          <div className="mt-4 grid gap-2" aria-label="Rematch vote status">
            {game.players.filter((player) => !player.bot).map((player) => (
              <div key={player.name} className="flex items-center justify-between gap-3 rounded-spade-md border border-spade-cream/10 bg-spade-bg/55 px-3 py-2">
                <span className="truncate text-sm text-spade-cream">{player.name}</span>
                {player.disconnected ? (
                  <Badge tone="danger">Left</Badge>
                ) : (
                  <Badge tone={player.votedRematch ? 'playing' : 'waiting'}>{player.votedRematch ? 'Voted' : 'Waiting'}</Badge>
                )}
              </div>
            ))}
          </div>
        </div>
      </div>
    </SectionPanel>
  )
}

function MatchStatsCard({ results, myDisplayName, practiceMode }: { results: GameResult[]; myDisplayName: string | null; practiceMode: boolean }) {
  if (practiceMode || !myDisplayName) return null
  const myResult = results.find((r) => r.player === myDisplayName)
  if (!myResult || myResult.xpDelta === undefined) return null

  const ratingDelta = myResult.ratingDelta ?? 0
  const ratingSign = ratingDelta >= 0 ? '+' : ''

  return (
    <div className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
      <h3 className="text-lg font-medium">Your match rewards</h3>
      <div className="mt-3 grid grid-cols-2 gap-3">
        <div className="rounded-spade-md border border-spade-cream/10 bg-spade-bg/45 p-3">
          <p className="font-mono text-xs uppercase text-spade-gray-3">XP gained</p>
          <p className="mt-1 text-lg font-medium text-spade-gold-light">+{myResult.xpDelta}</p>
          <p className="mt-0.5 font-mono text-xs text-spade-gray-2">Level {myResult.level}</p>
        </div>
        <div className="rounded-spade-md border border-spade-cream/10 bg-spade-bg/45 p-3">
          <p className="font-mono text-xs uppercase text-spade-gray-3">Rating</p>
          <p className={`mt-1 text-lg font-medium ${ratingDelta > 0 ? 'text-green-400' : ratingDelta < 0 ? 'text-red-400' : 'text-spade-cream'}`}>
            {ratingSign}{ratingDelta}
          </p>
          <p className="mt-0.5 font-mono text-xs text-spade-gray-2">{myResult.ratingAfter ?? '—'}</p>
        </div>
      </div>
    </div>
  )
}

function RevealedPenaltyCards({ results, myDisplayName, teamMode }: { results: GameResult[]; myDisplayName: string | null; teamMode: boolean }) {
  const myTeam = teamMode ? results.find((r) => r.player === myDisplayName)?.team : undefined

  return (
    <div className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
      <h3 className="text-lg font-medium">Revealed penalty cards</h3>
      <p className="mt-1 text-sm text-spade-gray-2">Face-down values are shown after the round ends.</p>
      <div className="mt-4 grid gap-3 md:grid-cols-2">
        {results.map((result) => (
          <RevealedPenaltyCardGroup key={result.player} result={result} teamMode={teamMode} isTeammate={teamMode && myTeam !== undefined && result.team === myTeam && result.player !== myDisplayName} />
        ))}
      </div>
    </div>
  )
}

function RevealedPenaltyCardGroup({ result, teamMode, isTeammate }: { result: GameResult; teamMode: boolean; isTeammate: boolean }) {
  const panelClassName = result.winner
    ? 'border-spade-gold/40 bg-spade-gold/10'
    : isTeammate
      ? getTeamColor(result.team ?? 0).badgeActive
      : 'border-spade-cream/10 bg-spade-bg/45'

  const teamBadgeClass = getTeamColor(result.team ?? 0).badge

  return (
    <div className={`rounded-spade-md border p-3 ${panelClassName}`}>
      <div className="mb-3 flex items-center justify-between gap-3">
        <div>
          <div className="flex items-center gap-2">
            <h4 className="font-medium">{result.player}</h4>
            {isTeammate ? <span className="text-[9px] font-medium text-blue-300">Teammate</span> : null}
          </div>
          <p className="font-mono text-xs text-spade-gray-2">Rank {result.rank} · {result.penalty} penalty</p>
        </div>
        <div className="flex items-center gap-1.5">
          {teamMode && result.team !== undefined ? (
            <span className={`inline-flex items-center gap-1.5 rounded-spade-pill border px-3 py-1 text-[11px] font-medium before:block before:size-1.5 before:rounded-full ${teamBadgeClass}`}>
              Team {result.team + 1}
            </span>
          ) : null}
          {result.winner ? <Badge tone="winner">Winner</Badge> : null}
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        {result.faceDownCards.length === 0 ? <span className="text-sm text-spade-gray-2">No penalty cards</span> : null}
        {result.faceDownCards.map((card, index) => (
          <div key={`${result.player}-${card.rank}-${card.suit}`} className="flex items-center gap-2 rounded-spade-sm border border-spade-cream/10 bg-spade-bg/70 px-2 py-1">
            <div className="anim-card-reveal" style={{ animationDelay: `calc(${index * 0.06}s * var(--anim-scale))` }}>
              <CardFace card={card} size="sm" interactive={false} ariaLabel={`${card.rank} of ${card.suit}`} />
            </div>
            <span className="grid gap-1">
              <span className="text-xs text-spade-cream">{card.rank} of {card.suit}</span>
              <span className="font-mono text-xs text-spade-gold-light">+{card.points}</span>
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

function useTurnClock(turnEndsAt: string | null, turnTimerSeconds: number): { label: string; percentRemaining: number } | null {
  // Re-render once per second so the derived clock value below stays current.
  // The clock itself is computed during render (not stored in state), which
  // avoids synchronously setting state inside the effect.
  const [, tick] = useState(0)

  useEffect(() => {
    if (!turnEndsAt) {
      return undefined
    }

    const interval = setInterval(() => {
      tick((value) => value + 1)
    }, 100)

    return () => clearInterval(interval)
  }, [turnEndsAt])

  return turnEndsAt ? getTurnClock(turnEndsAt, turnTimerSeconds) : null
}

function getTurnClock(turnEndsAt: string, turnTimerSeconds: number): { label: string; percentRemaining: number } {
  const endsAt = Date.parse(turnEndsAt)
  if (Number.isNaN(endsAt)) {
    return { label: 'Live', percentRemaining: 100 }
  }

  const milliseconds = Math.max(0, endsAt - Date.now())
  const seconds = Math.ceil(milliseconds / 1000)
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60
  // Use the room-configured duration so the progress bar is full at the start
  // of 30s, 90s, and 120s games instead of assuming every turn is 60s.
  const duration = Math.max(1, turnTimerSeconds)
  return {
    label: `${String(minutes).padStart(2, '0')}:${String(remainingSeconds).padStart(2, '0')}`,
    percentRemaining: Math.max(0, Math.min(100, (milliseconds / 1000 / duration) * 100)),
  }
}
