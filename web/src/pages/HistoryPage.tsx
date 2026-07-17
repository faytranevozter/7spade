import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { ApiError } from '../api/client'
import { getHistory, type HistoryGameDto } from '../api/history'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { SceneShell } from '../components/SceneShell'
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
  const totalPages = Math.max(1, Math.ceil(total / perPage))

  useEffect(() => {
    if (!isAuthenticated) {
      navigate('/auth', { replace: true })
    }
  }, [isAuthenticated, navigate])

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

      {/* Mobile card list */}
      <div className="grid gap-3 md:hidden">
        {games.map((game) => (
          <HistoryGameCard key={game.game_id} game={game} onResults={() => navigate(`/results/${game.game_id}`)} onReplay={() => navigate(`/replay/${game.game_id}`)} />
        ))}
        {!isLoading && games.length === 0 ? (
          <p className="rounded-spade-lg border border-spade-cream/12 bg-[#2b302d] px-4 py-8 text-center text-sm text-spade-gray-2">
            No completed games yet.
          </p>
        ) : null}
      </div>

      {/* Desktop table */}
      <div className="hidden overflow-x-auto rounded-spade-lg border border-spade-cream/12 bg-[#2b302d] md:block">
        <table aria-label="Game history" className="w-full min-w-[720px] text-sm">
          <thead className="bg-spade-cream/8 text-left font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">
            <tr>
              <th className="px-4 py-2">Room</th>
              <th className="px-2 py-2">Started</th>
              <th className="px-2 py-2">Result</th>
              <th className="px-2 py-2">Penalty</th>
              <th className="px-2 py-2">Rating</th>
              <th className="px-4 py-2">Finished</th>
              <th className="px-2 py-2">Results</th>
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
                <td className={`px-2 py-3 font-mono ${ratingClass(game.rating_delta)}`}>
                  {formatRatingDelta(game.rating_delta)}
                </td>
                <td className="px-4 py-3 text-xs text-spade-gray-2">{formatDate(game.finished_at)}</td>
                <td className="px-2 py-3">
                  {game.results_available ? (
                    <Button
                      variant="secondary"
                      size="xs"
                      onClick={() => navigate(`/results/${game.game_id}`)}
                    >
                      Results
                    </Button>
                  ) : (
                    <span className="text-xs text-spade-gray-3">—</span>
                  )}
                </td>
                <td className="px-2 py-3">
                  {game.replay_available ? (
                    <Button
                      variant="ghost"
                      size="xs"
                      onClick={() => navigate(`/replay/${game.game_id}`)}
                    >
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
                <td colSpan={8} className="px-4 py-8 text-center text-sm text-spade-gray-2">
                  No completed games yet.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      <div className="mt-4 flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
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

function HistoryGameCard({
  game,
  onResults,
  onReplay,
}: {
  game: HistoryGameDto
  onResults: () => void
  onReplay: () => void
}) {
  return (
    <article className="rounded-spade-lg border border-spade-cream/12 bg-[#2b302d] p-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="truncate text-sm font-medium text-spade-cream">{game.room_name || game.room_id}</h2>
          <p className="mt-0.5 font-mono text-[11px] text-spade-gray-3">{formatDate(game.finished_at)}</p>
        </div>
        <span className={`shrink-0 rounded-spade-pill border px-2 py-0.5 text-xs font-medium ${
          game.is_winner
            ? 'border-spade-gold/40 bg-spade-gold/15 text-spade-gold-light'
            : 'border-spade-cream/15 bg-spade-bg/50 text-spade-gray-2'
        }`}>
          {game.is_winner ? 'Winner' : `Rank ${game.rank}`}
        </span>
      </div>

      <dl className="mt-3 grid grid-cols-3 gap-2">
        <div className="rounded-spade-md border border-spade-cream/8 bg-spade-bg/40 px-2 py-1.5">
          <dt className="font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">Penalty</dt>
          <dd className="mt-0.5 font-mono text-sm text-spade-gold-light">{game.penalty_points}</dd>
        </div>
        <div className="rounded-spade-md border border-spade-cream/8 bg-spade-bg/40 px-2 py-1.5">
          <dt className="font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">Rating</dt>
          <dd className={`mt-0.5 font-mono text-sm ${ratingClass(game.rating_delta)}`}>
            {formatRatingDelta(game.rating_delta)}
          </dd>
        </div>
        <div className="rounded-spade-md border border-spade-cream/8 bg-spade-bg/40 px-2 py-1.5">
          <dt className="font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">Started</dt>
          <dd className="mt-0.5 text-xs text-spade-gray-2">{formatDate(game.started_at)}</dd>
        </div>
      </dl>

      {(game.results_available || game.replay_available) ? (
        <div className="mt-3 flex flex-wrap gap-2">
          {game.results_available ? (
            <Button variant="secondary" size="xs" onClick={onResults}>Results</Button>
          ) : null}
          {game.replay_available ? (
            <Button variant="ghost" size="xs" onClick={onReplay}>Replay</Button>
          ) : null}
        </div>
      ) : null}
    </article>
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

function formatRatingDelta(delta: number | null | undefined): string {
  if (delta == null) return '—'
  return `${delta >= 0 ? '+' : ''}${delta}`
}

function ratingClass(delta: number | null | undefined): string {
  if (delta == null) return 'text-spade-gray-2'
  if (delta > 0) return 'text-green-400'
  if (delta < 0) return 'text-red-400'
  return 'text-spade-gray-2'
}
