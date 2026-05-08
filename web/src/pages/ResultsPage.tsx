import { useNavigate, useParams } from 'react-router'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { PlayerAvatar } from '../components/PlayerAvatar'
import { ScoreTable } from '../components/ScoreTable'
import { SceneShell } from '../components/SceneShell'
import type { Player, Score } from '../types'

const scores: Score[] = [
  { rank: 1, player: 'Rini', cardsLeft: 0, penalty: 0, result: 'Winner', winner: true },
  { rank: 2, player: 'Santi', cardsLeft: 6, penalty: 24, result: 'Finished' },
  { rank: 3, player: 'Fahrur (you)', cardsLeft: 8, penalty: 12, result: 'Finished', me: true },
  { rank: 4, player: 'Budi', cardsLeft: 11, penalty: 52, result: 'Finished' },
]

const players: Player[] = [
  { name: 'Fahrur', initials: 'FA', cardsLeft: 0, faceDownCount: 2, tone: 'green', winner: true, votedRematch: true },
  { name: 'Budi', initials: 'BU', cardsLeft: 0, faceDownCount: 5, tone: 'gold', votedRematch: true },
  { name: 'Santi', initials: 'SA', cardsLeft: 0, faceDownCount: 4, tone: 'dark' },
  { name: 'Rini', initials: 'RI', cardsLeft: 0, faceDownCount: 3, tone: 'red', winner: true },
]

export function ResultsPage() {
  const { roomId } = useParams()
  const navigate = useNavigate()
  const votes = 2
  const total = 4

  return (
    <SceneShell
      title="Results and rematch"
      eyebrow={roomId ? `Room ${roomId}` : 'Game over + scoring'}
      action={<Badge tone="winner">{`${votes} / ${total} voted`}</Badge>}
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
        <div className="grid gap-4">
          <ScoreTable scores={scores} />
          <div className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
            <h3 className="text-lg font-medium">Revealed penalty cards</h3>
            <p className="mt-1 text-sm text-spade-gray-2">Face-down values are shown after the round ends.</p>
            <div className="mt-4 grid grid-cols-2 gap-3 md:grid-cols-4">
              {players.map((player) => (
                <PlayerAvatar key={player.name} player={player} />
              ))}
            </div>
          </div>
        </div>

        <div className="rounded-spade-lg border border-spade-gold/30 bg-spade-gold/10 p-4">
          <h3 className="text-lg font-medium">Rematch vote</h3>
          <p className="mt-1 text-sm text-spade-gray-2">
            The game restarts in the same room once every player votes for a rematch.
          </p>
          <div className="mt-4 grid gap-2">
            <Button onClick={() => navigate(`/game/${roomId ?? 'XKQP7'}`)}>Vote rematch</Button>
            <Button variant="secondary" onClick={() => navigate('/lobby')}>Leave room</Button>
            <Button variant="ghost" onClick={() => navigate('/history')}>View history</Button>
          </div>
          <div className="mt-4 h-2 overflow-hidden rounded-full bg-spade-bg/70">
            <div className="h-full rounded-full bg-spade-gold-light" style={{ width: `${(votes / total) * 100}%` }} />
          </div>
          <p className="mt-2 font-mono text-xs text-spade-gold-light">{votes} / {total} voted</p>
        </div>
      </div>
    </SceneShell>
  )
}
