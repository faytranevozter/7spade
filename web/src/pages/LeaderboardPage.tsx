import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { ApiError } from '../api/client'
import {
  DEFAULT_LEADERBOARD_SORT,
  getLeaderboard,
  getSeasons,
  isLeaderboardSort,
  LEADERBOARD_SORTS,
  type LeaderboardEntryDto,
  type LeaderboardSort,
  type SeasonDto,
} from '../api/stats'
import { Avatar } from '../components/Avatar'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { SceneShell } from '../components/SceneShell'
import { useAuth } from '../hooks/useAuth'
import { initialsForName } from '../game/cards'

const pageSizeOptions = [5, 10, 25, 50]

// The all-time leaderboard is the default scope; the value '' maps to no season
// query param. Concrete season ids (e.g. '2026-06') scope to that month.
const ALL_TIME = ''

// Maps each sortable column to its sort key so the header click and the active
// indicator stay in sync with the dropdown / URL.
const columnSorts: Record<string, LeaderboardSort> = {
  games: 'games_played',
  wins: 'total_wins',
  win_rate: 'win_rate',
  avg_penalty: 'avg_penalty',
  best: 'best_penalty',
  rating: 'rating',
}

export function LeaderboardPage() {
  const navigate = useNavigate()
  const { token } = useAuth()
  const [searchParams, setSearchParams] = useSearchParams()

  const sortParam = searchParams.get('sort')
  const sort: LeaderboardSort = isLeaderboardSort(sortParam) ? sortParam : DEFAULT_LEADERBOARD_SORT
  const pageParam = Number(searchParams.get('page'))
  const page = Number.isFinite(pageParam) && pageParam >= 1 ? pageParam : 1
  const season = searchParams.get('season') ?? ALL_TIME

  const [perPage, setPerPage] = useState(10)
  const [entries, setEntries] = useState<LeaderboardEntryDto[]>([])
  const [total, setTotal] = useState(0)
  const [minGames, setMinGames] = useState(0)
  const [seasons, setSeasons] = useState<SeasonDto[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const totalPages = Math.max(1, Math.ceil(total / perPage))

  function updateParams(next: { sort?: LeaderboardSort; page?: number; season?: string }) {
    setSearchParams(
      (current) => {
        const params = new URLSearchParams(current)
        if (next.sort !== undefined) {
          if (next.sort === DEFAULT_LEADERBOARD_SORT) params.delete('sort')
          else params.set('sort', next.sort)
        }
        if (next.page !== undefined) {
          if (next.page <= 1) params.delete('page')
          else params.set('page', String(next.page))
        }
        if (next.season !== undefined) {
          if (next.season === ALL_TIME) params.delete('season')
          else params.set('season', next.season)
        }
        return params
      },
      { replace: true },
    )
  }

  function setPage(updater: (current: number) => number) {
    updateParams({ page: updater(page) })
  }

  function setSort(nextSort: LeaderboardSort) {
    // Changing the sort resets to the first page so the user sees the top.
    updateParams({ sort: nextSort, page: 1 })
  }

  function setSeason(nextSeason: string) {
    // Changing the scope resets to the first page.
    updateParams({ season: nextSeason, page: 1 })
  }

  // Load the season list once so the selector can offer all-time + each season.
  useEffect(() => {
    let cancelled = false
    getSeasons(token)
      .then((response) => {
        if (!cancelled) setSeasons(response.seasons)
      })
      .catch(() => {
        // Non-fatal: the selector simply falls back to All-time only.
        if (!cancelled) setSeasons([])
      })
    return () => {
      cancelled = true
    }
  }, [token])

  useEffect(() => {
    let cancelled = false
    Promise.resolve()
      .then(() => {
        if (cancelled) return null
        setIsLoading(true)
        setError(null)
        return getLeaderboard(token, page, perPage, sort, season)
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
  }, [page, perPage, sort, season, token])

  const activeSeasonLabel = season === ALL_TIME ? 'All-time rankings' : seasonLabel(seasons, season)

  return (
    <SceneShell title="Leaderboard" eyebrow={activeSeasonLabel} action={<Badge tone="winner">{`Page ${page}`}</Badge>}>
      {error ? (
        <div className="mb-4 rounded-spade-md border border-spade-red/50 bg-spade-red-dark/30 px-4 py-3 text-sm text-spade-cream">
          {error}
        </div>
      ) : null}

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <label className="flex items-center gap-2 font-mono text-xs text-spade-gray-3">
          Season
          <select
            aria-label="Leaderboard season"
            className="rounded-spade-md border border-spade-cream/15 bg-spade-bg px-2 py-1 text-spade-cream"
            value={season}
            onChange={(event) => setSeason(event.target.value)}
          >
            <option value={ALL_TIME}>All-time</option>
            {seasons.map((option) => (
              <option key={option.id} value={option.id}>
                {option.label}{option.active ? ' (current)' : ''}
              </option>
            ))}
          </select>
        </label>
        <label className="flex items-center gap-2 font-mono text-xs text-spade-gray-3">
          Sort by
          <select
            aria-label="Sort leaderboard by"
            className="rounded-spade-md border border-spade-cream/15 bg-spade-bg px-2 py-1 text-spade-cream"
            value={sort}
            onChange={(event) => setSort(event.target.value as LeaderboardSort)}
          >
            {LEADERBOARD_SORTS.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>
        </label>
      </div>

      <div className="overflow-hidden rounded-spade-lg border border-spade-cream/12 bg-[#2b302d]">
        <table aria-label="Leaderboard" className="w-full text-sm">
          <thead className="bg-spade-cream/8 text-left font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">
            <tr>
              <th className="px-4 py-2">#</th>
              <th className="px-2 py-2">Player</th>
              <SortableHeader label="Rating" sortKey={columnSorts.rating} activeSort={sort} onSort={setSort} className="px-2 py-2" />
              <SortableHeader label="Games" sortKey={columnSorts.games} activeSort={sort} onSort={setSort} className="px-2 py-2" />
              <SortableHeader label="Wins" sortKey={columnSorts.wins} activeSort={sort} onSort={setSort} className="px-2 py-2" />
              <SortableHeader label="Win rate" sortKey={columnSorts.win_rate} activeSort={sort} onSort={setSort} className="px-2 py-2" />
              <SortableHeader label="Avg penalty" sortKey={columnSorts.avg_penalty} activeSort={sort} onSort={setSort} className="px-2 py-2" />
              <SortableHeader label="Best" sortKey={columnSorts.best} activeSort={sort} onSort={setSort} className="px-4 py-2" />
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
                    className="flex items-center gap-2 text-spade-cream underline-offset-2 hover:text-spade-gold hover:underline"
                  >
                    <Avatar
                      avatarUrl={entry.avatar_url}
                      initials={initialsForName(entry.display_name)}
                      alt={entry.display_name}
                      sizeClass="size-7"
                      className="text-xs"
                    />
                    {entry.display_name}
                  </button>
                </td>
                <td className={cellClass('rating', sort, 'text-spade-gold-light')}>{entry.rating}</td>
                <td className={cellClass('games', sort, 'text-spade-gray-2')}>{entry.games_played}</td>
                <td className={cellClass('wins', sort, 'text-spade-gray-2')}>{entry.wins}</td>
                <td className={cellClass('win_rate', sort)}>{formatPercent(entry.win_rate)}</td>
                <td className={cellClass('avg_penalty', sort, 'text-spade-gray-2')}>{entry.avg_penalty.toFixed(1)}</td>
                <td className={cellClass('best', sort, 'text-spade-gold-light', 'px-4')}>{entry.best_penalty ?? '—'}</td>
              </tr>
            ))}
            {!isLoading && entries.length === 0 ? (
              <tr className="border-t border-spade-cream/8">
                <td colSpan={8} className="px-4 py-8 text-center text-sm text-spade-gray-2">
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
        <div className="flex flex-wrap items-center gap-2">
          <label className="flex items-center gap-2 font-mono text-xs text-spade-gray-3">
            Rows
            <select
              className="rounded-spade-md border border-spade-cream/15 bg-spade-bg px-2 py-1 text-spade-cream"
              value={perPage}
              onChange={(event) => {
                setPerPage(Number(event.target.value))
                setPage(() => 1)
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

function SortableHeader({
  label,
  sortKey,
  activeSort,
  onSort,
  className,
}: {
  label: string
  sortKey: LeaderboardSort
  activeSort: LeaderboardSort
  onSort: (sort: LeaderboardSort) => void
  className?: string
}) {
  const isActive = activeSort === sortKey
  return (
    <th className={className} aria-sort={isActive ? 'descending' : 'none'}>
      <button
        type="button"
        onClick={() => onSort(sortKey)}
        className={`flex items-center gap-1 uppercase tracking-[0.06em] hover:text-spade-gold ${
          isActive ? 'text-spade-gold-light' : 'text-spade-gray-3'
        }`}
      >
        {label}
        <span aria-hidden="true" className="text-[8px]">{isActive ? '▾' : ''}</span>
      </button>
    </th>
  )
}

function cellClass(column: keyof typeof columnSorts, activeSort: LeaderboardSort, extra = '', padding = 'px-2'): string {
  const isActive = columnSorts[column] === activeSort
  return `${padding} py-3 font-mono ${isActive ? 'text-spade-gold-light' : extra}`.trim()
}

function getErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return fallback
}

function formatPercent(value: number): string {
  return `${(value * 100).toFixed(1)}%`
}

// seasonLabel resolves a season id to its human label for the scene eyebrow,
// falling back to the raw id (or a generic phrase) if the list hasn't loaded.
function seasonLabel(seasons: SeasonDto[], id: string): string {
  const match = seasons.find((s) => s.id === id)
  if (!match) return id ? `Season ${id}` : 'All-time rankings'
  return `${match.label}${match.active ? ' · current season' : ' season'}`
}
