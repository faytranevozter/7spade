import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { ApiError } from '../api/client'
import { getLeaderboard, type LeaderboardEntryDto } from '../api/stats'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { SceneShell } from '../components/SceneShell'
import { useAuth } from '../hooks/useAuth'

const perPage = 10

export function LeaderboardPage() {
  const navigate = useNavigate()
  const { token } = useAuth()
  const [page, setPage] = useState(1)
  const [entries, setEntries] = useState<LeaderboardEntryDto[]>([])
  const [total, setTotal] = useState(0)
  const [minGames, setMinGames] = useState(0)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const totalPages = Math.max(1, Math.ceil(total / perPage))

  useEffect(() => {
    let cancelled = false
    Promise.resolve()
      .then(() => {
        if (cancelled) return null
        setIsLoading(true)
        setError(null)
        return getLeaderboard(token, page, perPage)
      })
      .then((response) => {
        if (cancelled || response === null) return
        setEntries(response.entries)
        setTotal(response.total)
        setMinGames(response.min_games)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setError(getErrorMessage(err, 'Failed to load leaderboard'))
      })
      .finally(() => {
        if (cancelled) return
        setIsLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [page, token])

  return (
    <SceneShell title="Leaderboard" eyebrow="All-time rankings" action={<Badge tone="winner">{`Page ${page}`}</Badge>}>
      {error ? (
        <div className="mb-4 rounded-spade-md border border-spade-red/50 bg-spade-red-dark/30 px-4 py-3 text-sm text-spade-cream">
          {error}
        </div>
      ) : null}
      <div className="overflow-hidden rounded-spade-lg border border-spade-cream/12 bg-[#2b302d]">
        <table aria-label="Leaderboard" className="w-full text-sm">
          <thead className="bg-spade-cream/8 text-left font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">
            <tr>
              <th className="px-4 py-2">#</th>
              <th className="px-2 py-2">Player</th>
              <th className="px-2 py-2">Games</th>
              <th className="px-2 py-2">Win rate</th>
              <th className="px-2 py-2">Avg penalty</th>
              <th className="px-4 py-2">Best</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((entry) => (
              <tr key={entry.user_id} className="border-t border-spade-cream/8">
                <td className="px-4 py-3 font-mono text-spade-gold-light">{entry.rank}</td>
                <td className="px-2 py-3">
                  <button
                    type="button"
                    onClick={() => navigate(`/players/${entry.user_id}`)}
                    className="text-spade-cream underline-offset-2 hover:text-spade-gold hover:underline"
                  >
                    {entry.display_name}
                  </button>
                </td>
                <td className="px-2 py-3 text-spade-gray-2">{entry.games_played}</td>
                <td className="px-2 py-3 font-mono">{formatPercent(entry.win_rate)}</td>
                <td className="px-2 py-3 font-mono text-spade-gray-2">{entry.avg_penalty.toFixed(1)}</td>
                <td className="px-4 py-3 font-mono text-spade-gold-light">{entry.best_penalty ?? '—'}</td>
              </tr>
            ))}
            {!isLoading && entries.length === 0 ? (
              <tr className="border-t border-spade-cream/8">
                <td colSpan={6} className="px-4 py-8 text-center text-sm text-spade-gray-2">
                  No ranked players yet — play a few games to appear.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
        <p className="font-mono text-xs text-spade-gray-3">
          {isLoading
            ? 'Loading leaderboard...'
            : `${total} ranked players · page ${page} of ${totalPages} · min ${minGames} games to qualify`}
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

function getErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return fallback
}

function formatPercent(value: number): string {
  return `${(value * 100).toFixed(1)}%`
}
