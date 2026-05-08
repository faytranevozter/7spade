import { useState } from 'react'
import { useNavigate } from 'react-router'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { SceneShell } from '../components/SceneShell'

type HistoryGame = {
  gameId: string
  roomId: string
  startedAt: string
  finishedAt: string
  penaltyPoints: number
  rank: number
  isWinner: boolean
}

const games: HistoryGame[] = [
  {
    gameId: 'game-1',
    roomId: 'XKQP7',
    startedAt: '2026-05-09T10:00:00Z',
    finishedAt: '2026-05-09T10:20:00Z',
    penaltyPoints: 5,
    rank: 1,
    isWinner: true,
  },
  {
    gameId: 'game-2',
    roomId: 'PR7A2',
    startedAt: '2026-05-08T18:30:00Z',
    finishedAt: '2026-05-08T18:52:00Z',
    penaltyPoints: 24,
    rank: 2,
    isWinner: false,
  },
  {
    gameId: 'game-3',
    roomId: 'FNG44',
    startedAt: '2026-05-01T21:00:00Z',
    finishedAt: '2026-05-01T21:31:00Z',
    penaltyPoints: 52,
    rank: 4,
    isWinner: false,
  },
]

const perPage = 2

export function HistoryPage() {
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const totalPages = Math.max(1, Math.ceil(games.length / perPage))
  const visibleGames = games.slice((page - 1) * perPage, page * perPage)

  return (
    <SceneShell title="Game history" eyebrow="Logged-in player results" action={<Badge tone="waiting">{`Page ${page}`}</Badge>}>
      <div className="overflow-hidden rounded-spade-lg border border-spade-cream/12 bg-[#2b302d]">
        <table aria-label="Game history" className="w-full text-sm">
          <thead className="bg-spade-cream/8 text-left font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">
            <tr>
              <th className="px-4 py-2">Room</th>
              <th className="px-2 py-2">Started</th>
              <th className="px-2 py-2">Result</th>
              <th className="px-2 py-2">Penalty</th>
              <th className="px-4 py-2">Finished</th>
            </tr>
          </thead>
          <tbody>
            {visibleGames.map((game) => (
              <tr key={game.gameId} className="border-t border-spade-cream/8">
                <td className="px-4 py-3 font-mono text-xs">{game.roomId}</td>
                <td className="px-2 py-3 text-spade-gray-2">{formatDate(game.startedAt)}</td>
                <td className="px-2 py-3">{game.isWinner ? 'Winner' : `Rank ${game.rank}`}</td>
                <td className="px-2 py-3 font-mono text-spade-gold-light">{game.penaltyPoints}</td>
                <td className="px-4 py-3 text-xs text-spade-gray-2">{formatDate(game.finishedAt)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
        <p className="font-mono text-xs text-spade-gray-3">
          {games.length} games · page {page} of {totalPages}
        </p>
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" disabled={page <= 1} onClick={() => setPage((current) => Math.max(1, current - 1))}>
            Previous
          </Button>
          <Button variant="secondary" disabled={page >= totalPages} onClick={() => setPage((current) => Math.min(totalPages, current + 1))}>
            Next
          </Button>
          <Button variant="ghost" onClick={() => navigate('/lobby')}>Back to lobby</Button>
        </div>
      </div>
    </SceneShell>
  )
}

function formatDate(value: string): string {
  return new Date(value).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
