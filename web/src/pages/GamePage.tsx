import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { CardFace } from '../components/CardFace'
import { GameBoard } from '../components/GameBoard'
import { Modal } from '../components/Modal'
import { ScoreTable } from '../components/ScoreTable'
import { SectionPanel } from '../components/SectionPanel'
import { ToastStack } from '../components/ToastStack'
import { useAuth } from '../hooks/useAuth'
import { useGameSocket, type GameSocketState } from '../hooks/useGameSocket'
import type { Card, GameResult, Player } from '../types'

const connectionTone = {
  idle: 'waiting',
  connecting: 'waiting',
  open: 'playing',
  closed: 'danger',
  error: 'danger',
} as const

export function GamePage() {
  const { roomId } = useParams()
  const navigate = useNavigate()
  const { token } = useAuth()
  const game = useGameSocket(roomId, token)
  const hasValidMoves = game.hand.some((card) => card.playable)
  const showFaceDownModal = game.isMyTurn && game.hand.length > 0 && !hasValidMoves

  const visibleHand = useMemo(() => game.hand.map((card) => ({
    ...card,
    dimmed: game.isMyTurn && hasValidMoves && !card.playable,
  })), [game.hand, game.isMyTurn, hasValidMoves])

  const playCard = (card: Card) => {
    if (!game.isMyTurn || !card.playable) {
      return
    }

    game.sendPlayCard({ rank: card.rank, suit: card.suit, playable: card.playable })
  }

  const turnLabel = game.currentTurnName ? `${game.currentTurnName}'s turn` : 'Waiting...'
  const turnClock = useTurnClock(game.turnEndsAt)

  if (game.gameOver) {
    return <GameOverPanel roomId={roomId} game={game} />
  }

  return (
    <div className="relative flex min-h-[calc(100svh-60px)] flex-col">
      {/* Top bar: room info + connection status + actions menu */}
      <GameTopBar
        roomId={roomId}
        status={game.status}
        onReconnect={game.reconnect}
        onLeave={() => navigate('/lobby')}
        onHistory={() => navigate('/history')}
      />

      {/* Main game table area */}
      <div className="relative flex flex-1 flex-col items-center justify-center gap-3 px-3 py-3 sm:px-4">
        {/* Opponents row */}
        <OpponentsRow players={game.players} currentTurnName={game.currentTurnName} />

        {/* Game board */}
        <div className="w-full max-w-[820px]">
          <GameBoard rows={game.boardRows} />

          {/* Turn timer bar */}
          {turnClock ? (
            <div className="mt-2 rounded-spade-pill border border-spade-cream/10 bg-spade-bg/70 p-1" aria-label="Turn countdown">
              <div
                aria-label="Turn time remaining"
                className="h-1.5 rounded-spade-pill bg-gradient-to-r from-spade-gold-light to-spade-gold transition-[width] duration-500"
                style={{ width: `${turnClock.percentRemaining}%` }}
              />
            </div>
          ) : null}

          {/* Turn indicator */}
          <div className="mt-2 flex items-center justify-center gap-3">
            <Badge tone={game.isMyTurn ? 'playing' : 'waiting'}>{game.isMyTurn ? '⚡ Your turn' : turnLabel}</Badge>
            {turnClock ? (
              <span role="timer" aria-label="Turn timer" className="rounded-spade-pill border border-spade-gold-light/40 bg-spade-gold/15 px-2.5 py-0.5 font-mono text-xs text-spade-gold-light">
                {turnClock.label}
              </span>
            ) : null}
          </div>
        </div>

        {/* Player hand */}
        <PlayerHand
          cards={visibleHand}
          interactive={game.isMyTurn && hasValidMoves}
          onCardClick={playCard}
          isMyTurn={game.isMyTurn}
        />
      </div>

      <ToastStack toasts={game.toasts} />

      {showFaceDownModal ? (
        <Modal
          title="Place a face-down card"
          eyebrow="No valid moves"
          description="Select any card from your hand. It will be added to your face-down penalty pile for this round."
        >
          <div className="grid grid-cols-3 gap-3 sm:grid-cols-4">
            {game.hand.map((card) => (
              <CardFace
                key={`${card.rank}-${card.suit}`}
                card={card}
                ariaLabel={`Place ${card.rank} of ${card.suit} face down`}
                onClick={() => game.sendFaceDown(card)}
              />
            ))}
          </div>
        </Modal>
      ) : null}
    </div>
  )
}

function GameTopBar({
  roomId,
  status,
  onReconnect,
  onLeave,
  onHistory,
}: {
  roomId: string | undefined
  status: string
  onReconnect: () => void
  onLeave: () => void
  onHistory: () => void
}) {
  const [showMenu, setShowMenu] = useState(false)
  const statusLabel = status === 'open' ? 'Connected' : status

  return (
    <div className="flex items-center justify-between gap-3 border-b border-spade-cream/10 bg-spade-bg/80 px-4 py-2 backdrop-blur">
      <div className="flex items-center gap-2">
        <span className="font-mono text-xs text-spade-gray-3">{roomId ? `Room ${roomId}` : 'Room'}</span>
        <Badge tone={connectionTone[status as keyof typeof connectionTone] ?? 'waiting'}>{statusLabel}</Badge>
      </div>

      <div className="relative">
        <button
          type="button"
          onClick={() => setShowMenu(!showMenu)}
          className="rounded-spade-md border border-spade-cream/15 bg-spade-bg/60 px-3 py-1.5 text-xs text-spade-cream/80 transition hover:border-spade-cream/30 hover:text-spade-cream"
        >
          ⋯
        </button>
        {showMenu ? (
          <div className="absolute right-0 top-full z-30 mt-1 grid w-40 gap-1 rounded-spade-lg border border-spade-cream/15 bg-spade-bg p-2 shadow-lg">
            <button type="button" onClick={() => { onReconnect(); setShowMenu(false) }} className="rounded-spade-md px-3 py-1.5 text-left text-xs text-spade-cream/80 hover:bg-spade-green-mid/30">
              Reconnect
            </button>
            <button type="button" onClick={() => { onHistory(); setShowMenu(false) }} className="rounded-spade-md px-3 py-1.5 text-left text-xs text-spade-cream/80 hover:bg-spade-green-mid/30">
              History
            </button>
            <button type="button" onClick={() => { onLeave(); setShowMenu(false) }} className="rounded-spade-md px-3 py-1.5 text-left text-xs text-red-400/80 hover:bg-red-900/20">
              Leave room
            </button>
          </div>
        ) : null}
      </div>
    </div>
  )
}

function OpponentsRow({ players, currentTurnName }: { players: Player[]; currentTurnName: string | null }) {
  const opponents = players.filter((p) => p.name !== 'You')
  if (opponents.length === 0) return null

  return (
    <div className="flex w-full max-w-[820px] items-end justify-center gap-4 sm:gap-6">
      {opponents.map((player) => (
        <OpponentCard key={player.name} player={player} isCurrentTurn={player.name === currentTurnName} />
      ))}
    </div>
  )
}

function OpponentCard({ player, isCurrentTurn }: { player: Player; isCurrentTurn: boolean }) {
  const ringClass = isCurrentTurn ? 'ring-2 ring-spade-gold shadow-[0_0_12px_rgba(212,175,55,0.4)]' : ''
  const opacityClass = player.disconnected ? 'opacity-50' : ''

  const toneClasses: Record<Player['tone'], string> = {
    green: 'bg-spade-green-mid',
    gold: 'bg-[#7a5010]',
    dark: 'bg-[#2a2a3a]',
    red: 'bg-[#922b21]',
  }

  return (
    <div className={`flex flex-col items-center gap-1.5 rounded-spade-lg border border-spade-cream/10 bg-spade-bg/50 px-3 py-2 transition ${ringClass} ${opacityClass}`}>
      <div className={`grid size-9 place-items-center rounded-full ${toneClasses[player.tone]} text-xs font-medium text-spade-cream`}>
        {player.initials}
      </div>
      <span className="max-w-[80px] truncate text-xs font-medium text-spade-cream">{player.name}</span>
      <div className="flex items-center gap-2 text-[10px] text-spade-gray-3">
        <span title="Cards in hand">🃏 {player.cardsLeft}</span>
        <span title="Face-down cards">⬇ {player.faceDownCount}</span>
      </div>
      {player.disconnected ? <span className="text-[9px] text-red-400">Disconnected</span> : null}
    </div>
  )
}

function PlayerHand({
  cards,
  interactive,
  onCardClick,
  isMyTurn,
}: {
  cards: Card[]
  interactive: boolean
  onCardClick: (card: Card) => void
  isMyTurn: boolean
}) {
  if (cards.length === 0) return null

  const totalCards = cards.length
  const maxRotation = Math.min(totalCards * 2, 20)

  return (
    <div className="w-full max-w-[820px]">
      <div className="flex items-center justify-between px-1 pb-1">
        <span className="text-xs font-medium text-spade-cream/70">Your hand</span>
        <span className="font-mono text-[10px] text-spade-gray-3">{cards.length} cards</span>
      </div>
      <div className="relative flex items-end justify-center pb-2 pt-4">
        {cards.map((card, index) => {
          const centerOffset = index - (totalCards - 1) / 2
          const rotation = (centerOffset / ((totalCards - 1) / 2 || 1)) * maxRotation
          const translateY = Math.abs(centerOffset) * 2

          return (
            <div
              key={`${card.rank}-${card.suit}-${index}`}
              className="-ml-5 first:ml-0 transition-transform duration-150"
              style={{
                transform: `rotate(${rotation}deg) translateY(${translateY}px)`,
                zIndex: index + 1,
              }}
            >
              <CardFace
                card={card}
                interactive={interactive}
                onClick={interactive && card.playable ? () => onCardClick(card) : undefined}
              />
            </div>
          )
        })}
      </div>
      {isMyTurn ? (
        <p className="text-center text-xs text-spade-gold/80">
          {interactive ? 'Play a highlighted card to extend a suit' : 'No valid moves — choose a penalty card'}
        </p>
      ) : null}
    </div>
  )
}

function GameOverPanel({ roomId, game }: { roomId: string | undefined; game: GameSocketState }) {
  const navigate = useNavigate()
  const hasSharedWin = game.results.filter((result) => result.winner).length > 1
  const winnerLabel = hasSharedWin ? 'Shared winner' : 'Winner'
  const rematchProgress = (game.rematchVotes / game.rematchTotal) * 100
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
      eyebrow={roomId ? `Room ${roomId}` : 'Game over + scoring'}
      action={<Badge tone="winner">Round over</Badge>}
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_340px]">
        <div className="grid gap-4">
          <ScoreTable scores={scores} winnerLabel={winnerLabel} />
          <RevealedPenaltyCards results={game.results} />
        </div>

        <div className="rounded-spade-lg border border-spade-gold/30 bg-spade-gold/10 p-4">
          <h3 className="text-lg font-medium">Rematch vote</h3>
          <p className="mt-1 text-sm text-spade-gray-2">
            The game restarts in the same room once every player votes for a rematch.
          </p>
          <div className="mt-4 grid gap-2">
            <Button onClick={game.sendRematchVote}>Vote rematch</Button>
            <Button variant="secondary" onClick={() => navigate('/lobby')}>Leave room</Button>
            <Button variant="ghost" onClick={() => navigate('/history')}>View history</Button>
          </div>
          <div className="mt-4 h-2 overflow-hidden rounded-full bg-spade-bg/70">
            <div className="h-full rounded-full bg-spade-gold-light" style={{ width: `${rematchProgress}%` }} />
          </div>
          <p className="mt-2 font-mono text-xs text-spade-gold-light">{game.rematchVotes} / {game.rematchTotal} voted</p>
          <div className="mt-4 grid gap-2" aria-label="Rematch vote status">
            {game.players.map((player) => (
              <div key={player.name} className="flex items-center justify-between gap-3 rounded-spade-md border border-spade-cream/10 bg-spade-bg/55 px-3 py-2">
                <span className="truncate text-sm text-spade-cream">{player.name}</span>
                <Badge tone={player.votedRematch ? 'playing' : 'waiting'}>{player.votedRematch ? 'Voted' : 'Waiting'}</Badge>
              </div>
            ))}
          </div>
        </div>
      </div>
    </SectionPanel>
  )
}

function RevealedPenaltyCards({ results }: { results: GameResult[] }) {
  return (
    <div className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
      <h3 className="text-lg font-medium">Revealed penalty cards</h3>
      <p className="mt-1 text-sm text-spade-gray-2">Face-down values are shown after the round ends.</p>
      <div className="mt-4 grid gap-3 md:grid-cols-2">
        {results.map((result) => <RevealedPenaltyCardGroup key={result.player} result={result} />)}
      </div>
    </div>
  )
}

function RevealedPenaltyCardGroup({ result }: { result: GameResult }) {
  const panelClassName = result.winner
    ? 'border-spade-gold/40 bg-spade-gold/10'
    : 'border-spade-cream/10 bg-spade-bg/45'

  return (
    <div className={`rounded-spade-md border p-3 ${panelClassName}`}>
      <div className="mb-3 flex items-center justify-between gap-3">
        <div>
          <h4 className="font-medium">{result.player}</h4>
          <p className="font-mono text-xs text-spade-gray-2">Rank {result.rank} · {result.penalty} penalty</p>
        </div>
        {result.winner ? <Badge tone="winner">Winner</Badge> : null}
      </div>

      <div className="flex flex-wrap gap-2">
        {result.faceDownCards.length === 0 ? <span className="text-sm text-spade-gray-2">No penalty cards</span> : null}
        {result.faceDownCards.map((card) => (
          <div key={`${result.player}-${card.rank}-${card.suit}`} className="flex items-center gap-2 rounded-spade-sm border border-spade-cream/10 bg-spade-bg/70 px-2 py-1">
            <CardFace card={card} size="sm" interactive={false} ariaLabel={`${card.rank} of ${card.suit}`} />
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

function useTurnClock(turnEndsAt: string | null): { label: string; percentRemaining: number } | null {
  const [clock, setClock] = useState<{ label: string; percentRemaining: number } | null>(
    turnEndsAt ? getTurnClock(turnEndsAt) : null
  )

  useEffect(() => {
    if (!turnEndsAt) {
      setClock(null)
      return
    }

    setClock(getTurnClock(turnEndsAt))

    const interval = setInterval(() => {
      setClock(getTurnClock(turnEndsAt))
    }, 1000)

    return () => clearInterval(interval)
  }, [turnEndsAt])

  return clock
}

function getTurnClock(turnEndsAt: string): { label: string; percentRemaining: number } {
  const endsAt = Date.parse(turnEndsAt)
  if (Number.isNaN(endsAt)) {
    return { label: 'Live', percentRemaining: 100 }
  }

  const seconds = Math.max(0, Math.ceil((endsAt - Date.now()) / 1000))
  return {
    label: `00:${String(seconds).padStart(2, '0')}`,
    percentRemaining: Math.max(0, Math.min(100, Math.round((seconds / 60) * 100))),
  }
}
