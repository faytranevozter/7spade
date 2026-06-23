import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { ApiError } from '../api/client'
import { getHistory, type HistoryGameDto } from '../api/history'
import { getMyStats, type UserStatsDto } from '../api/stats'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { SceneShell } from '../components/SceneShell'
import { SectionPanel } from '../components/SectionPanel'
import { StatCards } from '../components/StatCards'
import { useAuth } from '../hooks/useAuth'

const pageSizeOptions = [5, 10, 25, 50]

export function HistoryPage() {
  const navigate = useNavigate()
  const { token, isAuthenticated } = useAuth()
  const [page, setPage] = useState(1)
  const [perPage, setPerPage] = useState(10)
  const [games, setGames] = useState<HistoryGameDto[]>([])
  const [total, setTotal] = useState(0)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [stats, setStats] = useState<UserStatsDto | null>(null)
  const totalPages = Math.max(1, Math.ceil(total / perPage))

  useEffect(() => {
    if (!isAuthenticated) {
      navigate('/auth', { replace: true })
      return
    }
    let cancelled = false
    getMyStats(token)
      .then((response) => {
        if (cancelled) return
        setStats(response)
      })
      .catch(() => {
        // Stats are supplementary; a failure here shouldn't block the history list.
      })
    return () => {
      cancelled = true
    }
  }, [isAuthenticated, navigate, token])

  useEffect(() => {
    if (!isAuthenticated) {
      navigate('/auth', { replace: true })
      return
    }
    let cancelled = false
    Promise.resolve()
      .then(() => {
        if (cancelled) return null
        setIsLoading(true)
        setError(null)
        return getHistory(token, page, perPage)
      })
      .then((response) => {
        if (cancelled || response === null) return
        setGames(response.games)
        setTotal(response.total)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setError(getErrorMessage(err, 'Failed to load game history'))
      })
      .finally(() => {
        if (cancelled) return
        setIsLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [isAuthenticated, navigate, page, perPage, token])

  return (
    <SceneShell title="Game history" eyebrow="Logged-in player results" action={<Badge tone="waiting">{`Page ${page}`}</Badge>}>
      {error ? (
        <div className="mb-4 rounded-spade-md border border-spade-red/50 bg-spade-red-dark/30 px-4 py-3 text-sm text-spade-cream">
          {error}
        </div>
      ) : null}
      {stats ? (
        <div className="mb-4">
          <SectionPanel title="Your stats" eyebrow="Lifetime totals">
            <StatCards stats={stats} />
            {!stats.qualified ? (
              <p className="mt-3 font-mono text-xs text-spade-gray-3">
                Play more games to join the leaderboard.
              </p>
            ) : null}
          </SectionPanel>
        </div>
      ) : null}
      <div className="overflow-hidden rounded-spade-lg border border-spade-cream/12 bg-[#2b302d]">
        <table aria-label="Game history" className="w-full text-sm">
          <thead className="bg-spade-cream/8 text-left font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">
            <tr>
              <th className="px-4 py-2">Room</th>
              <th className="px-2 py-2">Started</th>
              <th className="px-2 py-2">Result</th>
              <th className="px-2 py-2">Penalty</th>
              <th className="px-2 py-2">Rating</th>
              <th className="px-4 py-2">Finished</th>
              <th className="px-2 py-2">Replay</th>
            </tr>
          </thead>
          <tbody>
            {games.map((game) => (
              <tr key={game.game_id} className="border-t border-spade-cream/8">
                <td className="max-w-[160px] truncate px-4 py-3 text-spade-cream">{game.room_name || game.room_id}</td>
                <td className="px-2 py-3 text-spade-gray-2">{formatDate(game.started_at)}</td>
                <td className="px-2 py-3">{game.is_winner ? 'Winner' : `Rank ${game.rank}`}</td>
                <td className="px-2 py-3 font-mono text-spade-gold-light">{game.penalty_points}</td>
                <td className={`px-2 py-3 font-mono ${game.rating_delta != null ? (game.rating_delta > 0 ? 'text-green-400' : game.rating_delta < 0 ? 'text-red-400' : 'text-spade-gray-2') : 'text-spade-gray-2'}`}>
                  {game.rating_delta != null ? `${game.rating_delta >= 0 ? '+' : ''}${game.rating_delta}` : '—'}
                </td>
                <td className="px-4 py-3 text-xs text-spade-gray-2">{formatDate(game.finished_at)}</td>
                <td className="px-2 py-3">
                  {game.replay_available ? (
                    <Button variant="ghost" onClick={() => navigate(`/replay/${game.game_id}`)}>
                      Replay
                    </Button>
                  ) : (
                    <span className="text-xs text-spade-gray-3">—</span>
                  )}
                </td>
              </tr>
            ))}
            {!isLoading && games.length === 0 ? (
              <tr className="border-t border-spade-cream/8">
                <td colSpan={7} className="px-4 py-8 text-center text-sm text-spade-gray-2">
                  No completed games yet.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
        <p className="font-mono text-xs text-spade-gray-3">
          {isLoading ? 'Loading games...' : `${total} games · page ${page} of ${totalPages}`}
        </p>
        <div className="flex flex-wrap items-center gap-2">
          <label className="flex items-center gap-2 font-mono text-xs text-spade-gray-3">
            Rows
            <select
              className="rounded-spade-md border border-spade-cream/15 bg-spade-bg px-2 py-1 text-spade-cream"
              value={perPage}
              onChange={(event) => {
                setPerPage(Number(event.target.value))
                setPage(1)
              }}
            >
              {pageSizeOptions.map((option) => (
                <option key={option} value={option}>{option}</option>
              ))}
            </select>
          </label>
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

function formatDate(value: string): string {
  return new Date(value).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
