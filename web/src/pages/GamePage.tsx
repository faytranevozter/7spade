import { useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { CardStack } from '../components/CardStack'
import { GameBoard } from '../components/GameBoard'
import { PlayerAvatar } from '../components/PlayerAvatar'
import { SectionPanel } from '../components/SectionPanel'
import { ToastStack } from '../components/ToastStack'
import type { BoardRow, Card, Player, Toast } from '../types'

const players: Player[] = [
  { name: 'Fahrur', initials: 'FA', cardsLeft: 8, faceDownCount: 2, tone: 'green', active: true },
  { name: 'Budi', initials: 'BU', cardsLeft: 11, faceDownCount: 0, tone: 'gold' },
  { name: 'Santi', initials: 'SA', cardsLeft: 6, faceDownCount: 3, tone: 'dark' },
  { name: 'Rini', initials: 'RI', cardsLeft: 0, faceDownCount: 1, tone: 'red', winner: true },
]

const hand: Card[] = [
  { rank: '2', suit: 'Hearts' },
  { rank: '3', suit: 'Clubs' },
  { rank: '4', suit: 'Hearts' },
  { rank: '5', suit: 'Diamonds' },
  { rank: '6', suit: 'Spades', playable: true },
  { rank: '7', suit: 'Clubs' },
  { rank: '8', suit: 'Diamonds' },
  { rank: '9', suit: 'Hearts' },
  { rank: '10', suit: 'Spades' },
  { rank: 'J', suit: 'Clubs' },
  { rank: 'Q', suit: 'Diamonds' },
  { rank: 'K', suit: 'Hearts' },
  { rank: 'A', suit: 'Spades' },
]

const boardRows: BoardRow[] = [
  { suit: 'Hearts', cards: ['A', '2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K'] },
  { suit: 'Spades', cards: [null, null, null, null, null, '7', '8', null, null, null, null, null, null] },
  { suit: 'Diamonds', cards: [null, null, null, null, null, '7', '8', '9', '10', null, null, null, null] },
  { suit: 'Clubs', cards: [null, null, null, null, null, null, null, null, null, null, null, null, null], closed: true },
]

const toasts: Toast[] = [
  { tone: 'success', title: 'Card played', body: '8 Hearts placed on the Hearts row' },
  { tone: 'warn', title: '10 seconds left', body: 'Make your move or prepare a penalty card' },
  { tone: 'info', title: "Budi's turn", body: 'Waiting for another player to move' },
]

export function GamePage() {
  const { roomId } = useParams()
  const navigate = useNavigate()
  const [selectedCard, setSelectedCard] = useState<Card | null>(hand.find((card) => card.playable) ?? null)

  const visibleHand = useMemo(() => hand.map((card) => ({
    ...card,
    selected: selectedCard?.rank === card.rank && selectedCard.suit === card.suit,
  })), [selectedCard])

  return (
    <SectionPanel
      title="Live game table"
      eyebrow={roomId ? `Room ${roomId}` : 'Room preview'}
      action={
        <div className="flex flex-wrap gap-2">
          <Badge tone="playing">Your turn</Badge>
          <Badge tone="playing">Connected</Badge>
          <span className="rounded-spade-pill border border-spade-gold-light/40 bg-spade-gold/15 px-3 py-1 font-mono text-xs text-spade-gold-light">
            00:18
          </span>
        </div>
      }
    >
      <div className="grid gap-4">
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          {players.map((player) => (
            <PlayerAvatar key={player.name} player={player} />
          ))}
        </div>

        <GameBoard rows={boardRows} />

        <CardStack
          cards={visibleHand}
          interactive
          onCardClick={setSelectedCard}
          meta={`${hand.length} cards`}
        />

        <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_340px]">
          <div className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
            <div className="mb-3 flex items-center justify-between gap-3">
              <h3 className="text-lg font-medium">Table state</h3>
              <Badge tone="playing">Act now</Badge>
            </div>
            <p className="text-sm text-spade-gray-2">
              {selectedCard ? `${selectedCard.rank} of ${selectedCard.suit} selected` : 'Select a card to preview the move.'}
            </p>
          </div>

          <div className="rounded-spade-lg border border-spade-cream/10 bg-spade-bg/50 p-4">
            <div className="mb-3 flex items-center justify-between">
              <h3 className="text-lg font-medium">Actions</h3>
              <Badge tone="waiting">Preview</Badge>
            </div>
            <div className="grid grid-cols-2 gap-2">
              <Button onClick={() => navigate(`/results/${roomId ?? 'XKQP7'}`)}>Play card</Button>
              <Button variant="secondary" onClick={() => navigate(`/results/${roomId ?? 'XKQP7'}`)}>Face down</Button>
              <Button variant="ghost" onClick={() => navigate('/history')}>History</Button>
              <Button variant="danger" onClick={() => navigate('/lobby')}>Leave room</Button>
            </div>
          </div>
        </div>

        <ToastStack toasts={toasts} />
      </div>
    </SectionPanel>
  )
}
