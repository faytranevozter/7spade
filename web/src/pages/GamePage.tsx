import { useMemo } from 'react'
import { useNavigate, useParams } from 'react-router'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { CardFace } from '../components/CardFace'
import { CardStack } from '../components/CardStack'
import { GameBoard } from '../components/GameBoard'
import { Modal } from '../components/Modal'
import { PlayerAvatar } from '../components/PlayerAvatar'
import { SectionPanel } from '../components/SectionPanel'
import { ToastStack } from '../components/ToastStack'
import { useAuth } from '../hooks/useAuth'
import { useGameSocket } from '../hooks/useGameSocket'
import type { Card } from '../types'

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

  const statusLabel = game.status === 'open' ? 'Connected' : game.status
  const turnLabel = game.currentTurnName ? `Turn: ${game.currentTurnName}` : 'Waiting for turn'
	const turnClock = game.turnEndsAt ? getTurnClock(game.turnEndsAt) : null

  return (
    <SectionPanel
      title="Live game table"
      eyebrow={roomId ? `Room ${roomId}` : 'Room'}
      action={
        <div className="flex flex-wrap gap-2">
          <Badge tone={game.isMyTurn ? 'playing' : 'waiting'}>{game.isMyTurn ? 'Your turn' : turnLabel}</Badge>
          <Badge tone={connectionTone[game.status]}>{statusLabel}</Badge>
          {turnClock ? (
            <span role="timer" aria-label="Turn timer" className="rounded-spade-pill border border-spade-gold-light/40 bg-spade-gold/15 px-3 py-1 font-mono text-xs text-spade-gold-light">
              {turnClock.label}
            </span>
          ) : null}
        </div>
      }
    >
      <div className="grid gap-4">
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          {game.players.map((player) => (
            <PlayerAvatar key={player.name} player={player} />
          ))}
        </div>

        <GameBoard rows={game.boardRows} />

        {turnClock ? (
          <div className="rounded-spade-pill border border-spade-cream/10 bg-spade-bg/70 p-1" aria-label="Turn countdown">
            <div
              aria-label="Turn time remaining"
              className="h-2 rounded-spade-pill bg-gradient-to-r from-spade-gold-light to-spade-gold transition-[width] duration-500"
              style={{ width: `${turnClock.percentRemaining}%` }}
            />
          </div>
        ) : null}

        <CardStack
          cards={visibleHand}
          interactive={game.isMyTurn && hasValidMoves}
          onCardClick={playCard}
          meta={`${game.hand.length} cards · ${turnLabel}`}
        />

        <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_340px]">
          <div className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
            <div className="mb-3 flex items-center justify-between gap-3">
              <h3 className="text-lg font-medium">Table state</h3>
              <Badge tone={game.isMyTurn ? 'playing' : 'waiting'}>{turnLabel}</Badge>
            </div>
            <p className="text-sm text-spade-gray-2">
              {game.isMyTurn
                ? hasValidMoves ? 'Play a highlighted card to extend an open suit.' : 'No valid moves. Choose a penalty card to place face down.'
                : 'Waiting for the active player to move.'}
            </p>
          </div>

          <div className="rounded-spade-lg border border-spade-cream/10 bg-spade-bg/50 p-4">
            <div className="mb-3 flex items-center justify-between">
              <h3 className="text-lg font-medium">Actions</h3>
              <Badge tone={game.gameOver ? 'winner' : 'waiting'}>{game.gameOver ? 'Round over' : 'Live'}</Badge>
            </div>
            <div className="grid grid-cols-2 gap-2">
              <Button variant="secondary" onClick={game.reconnect}>Reconnect</Button>
              <Button variant="ghost" onClick={() => navigate('/history')}>History</Button>
              <Button variant="danger" onClick={() => navigate('/lobby')}>Leave room</Button>
            </div>
          </div>
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
    </SectionPanel>
  )
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
