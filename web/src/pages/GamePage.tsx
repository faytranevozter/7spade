import { useMemo } from 'react'
import { useNavigate, useParams } from 'react-router'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { CardFace } from '../components/CardFace'
import { CardStack } from '../components/CardStack'
import { GameBoard } from '../components/GameBoard'
import { Modal } from '../components/Modal'
import { PlayerAvatar } from '../components/PlayerAvatar'
import { ScoreTable } from '../components/ScoreTable'
import { SectionPanel } from '../components/SectionPanel'
import { ToastStack } from '../components/ToastStack'
import { useAuth } from '../hooks/useAuth'
import { useGameSocket, type GameSocketState } from '../hooks/useGameSocket'
import type { Card, GameResult } from '../types'

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
  const tableStateMessage = getTableStateMessage(game.isMyTurn, hasValidMoves)

  if (game.gameOver) {
    return <GameOverPanel roomId={roomId} game={game} />
  }

  return (
    <SectionPanel
      title="Live game table"
      eyebrow={roomId ? `Room ${roomId}` : 'Room'}
      action={
        <div className="flex flex-wrap gap-2">
          <Badge tone={game.isMyTurn ? 'playing' : 'waiting'}>{game.isMyTurn ? 'Your turn' : turnLabel}</Badge>
          <Badge tone={connectionTone[game.status]}>{statusLabel}</Badge>
          {game.turnEndsAt ? (
            <span className="rounded-spade-pill border border-spade-gold-light/40 bg-spade-gold/15 px-3 py-1 font-mono text-xs text-spade-gold-light">
              {formatTurnClock(game.turnEndsAt)}
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
              {tableStateMessage}
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

function getTableStateMessage(isMyTurn: boolean, hasValidMoves: boolean): string {
  if (!isMyTurn) {
    return 'Waiting for the active player to move.'
  }

  if (!hasValidMoves) {
    return 'No valid moves. Choose a penalty card to place face down.'
  }

  return 'Play a highlighted card to extend an open suit.'
}

function formatTurnClock(turnEndsAt: string): string {
  const endsAt = Date.parse(turnEndsAt)
  if (Number.isNaN(endsAt)) {
    return 'Live'
  }

  const seconds = Math.max(0, Math.ceil((endsAt - Date.now()) / 1000))
  return `00:${String(seconds).padStart(2, '0')}`
}
