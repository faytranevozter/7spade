import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { searchGames, type Game, type GameFilters } from '../api/games'
import { Notice } from './Feedback'
import { EmptyState, FilterField, Pagination } from './InvestigationUI'
import { formatDateTime, formatLabel, shortID } from './formatters'
import { AdminPage, AdminPageHeader, AdminPanel } from './AdminPage'

export function GameInvestigation({ token }: { token: string }) {
  const [filters, setFilters] = useState<GameFilters>({})
  const [games, setGames] = useState<Game[]>([])
  const [total, setTotal] = useState(0)
  const [message, setMessage] = useState('')
  const [loading, setLoading] = useState(true)
  const [offset, setOffset] = useState(0)
  const pageSize = 50

  useEffect(() => {
    let cancelled = false
    const request = window.setTimeout(() => {
      setLoading(true)
      searchGames(token, filters, pageSize, offset)
        .then((page) => {
          if (!cancelled) {
            setGames(page.games ?? [])
            setTotal(page.total ?? page.games?.length ?? 0)
            setMessage('')
          }
        })
        .catch((error: unknown) => {
          if (!cancelled)
            setMessage(
              error instanceof Error ? error.message : 'Failed to search games',
            )
        })
        .finally(() => {
          if (!cancelled) setLoading(false)
        })
    }, 0)
    return () => {
      cancelled = true
      window.clearTimeout(request)
    }
  }, [filters, offset, token])

  function update(key: keyof GameFilters, value: string) {
    setFilters((current) => ({ ...current, [key]: value }))
    setOffset(0)
  }

  function clearFilters() {
    setFilters({})
    setOffset(0)
  }

  const activeFilters = Object.values(filters).filter(Boolean).length
  const pageStart = total === 0 ? 0 : offset + 1
  const pageEnd = Math.min(
    offset + games.length,
    total || offset + games.length,
  )

  return (
    <AdminPage labelledBy="games-heading">
      <AdminPageHeader
        eyebrow="Archive / Game investigations"
        title="Games"
        titleId="games-heading"
        description="Search recorded games, trace a result from room to final move, then preserve review context without altering the record."
        actions={
          <div
            className="border-admin-accent/35 bg-admin-accent/7 px-admin-17 min-w-41.25 rounded-xl border py-4 text-right max-[700px]:min-w-0 max-[700px]:text-left"
            aria-label={`${total} games found`}
          >
            <strong className="text-admin-accent-bright text-admin-count block font-mono font-medium">
              {total}
            </strong>
            <span className="text-admin-muted text-admin-field">
              matching records
            </span>
          </div>
        }
      />

      <div className="mt-8 grid grid-cols-[minmax(245px,310px)_minmax(0,1fr)] items-start gap-8 max-[980px]:grid-cols-1">
        <aside
          className="sticky top-6 max-[980px]:static"
          aria-labelledby="filter-heading"
        >
          <AdminPanel>
            <div className="flex items-center justify-between gap-4">
              <div>
                <p className="text-admin-accent text-admin-note m-0 font-mono font-medium tracking-[0.13em] uppercase">
                  Query builder
                </p>
                <h2
                  id="filter-heading"
                  className="text-admin-ink-strong mt-admin-4 text-admin-section mb-0 font-semibold"
                >
                  Narrow the archive
                </h2>
              </div>
              {activeFilters > 0 ? (
                <span className="bg-admin-accent text-admin-button-ink text-admin-control grid size-6.5 place-items-center rounded-full font-mono font-medium">
                  {activeFilters}
                </span>
              ) : null}
            </div>

            <div className="mt-admin-18 gap-admin-15 grid max-[980px]:grid-cols-2 max-[700px]:grid-cols-1">
              <FilterField label="Game ID">
                <input
                  value={filters.id ?? ''}
                  onChange={(event) => update('id', event.target.value)}
                  placeholder="UUID or exact ID"
                  className="border-admin-border-input bg-admin-canvas text-admin-field text-admin-ink-strong placeholder:text-admin-muted-subtle focus:border-admin-accent focus:bg-admin-surface-raised focus:shadow-admin-focus rounded-admin-input py-admin-10 w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
                />
              </FilterField>
              <FilterField label="Room ID">
                <input
                  value={filters.room_id ?? ''}
                  onChange={(event) => update('room_id', event.target.value)}
                  placeholder="Room UUID"
                  className="border-admin-border-input bg-admin-canvas text-admin-field text-admin-ink-strong placeholder:text-admin-muted-subtle focus:border-admin-accent focus:bg-admin-surface-raised focus:shadow-admin-focus rounded-admin-input py-admin-10 w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
                />
              </FilterField>
              <FilterField label="Player ID">
                <input
                  value={filters.player_id ?? ''}
                  onChange={(event) => update('player_id', event.target.value)}
                  placeholder="Player UUID"
                  className="border-admin-border-input bg-admin-canvas text-admin-field text-admin-ink-strong placeholder:text-admin-muted-subtle focus:border-admin-accent focus:bg-admin-surface-raised focus:shadow-admin-focus rounded-admin-input py-admin-10 w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
                />
              </FilterField>
              <div className="gap-admin-9 grid grid-cols-2">
                <FilterField label="Mode">
                  <input
                    value={filters.mode ?? ''}
                    onChange={(event) => update('mode', event.target.value)}
                    placeholder="classic"
                    className="border-admin-border-input bg-admin-canvas text-admin-field text-admin-ink-strong placeholder:text-admin-muted-subtle focus:border-admin-accent focus:bg-admin-surface-raised focus:shadow-admin-focus rounded-admin-input py-admin-10 w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
                  />
                </FilterField>
                <FilterField label="Completion">
                  <select
                    value={filters.completion ?? ''}
                    onChange={(event) =>
                      update('completion', event.target.value)
                    }
                    className="border-admin-border-input bg-admin-canvas text-admin-field text-admin-ink-strong placeholder:text-admin-muted-subtle focus:border-admin-accent focus:bg-admin-surface-raised focus:shadow-admin-focus rounded-admin-input py-admin-10 w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
                  >
                    <option value="">Any state</option>
                    <option value="completed">Completed</option>
                  </select>
                </FilterField>
              </div>
              <FilterField label="Season">
                <input
                  value={filters.season_id ?? ''}
                  onChange={(event) => update('season_id', event.target.value)}
                  placeholder="Season UUID"
                  className="border-admin-border-input bg-admin-canvas text-admin-field text-admin-ink-strong placeholder:text-admin-muted-subtle focus:border-admin-accent focus:bg-admin-surface-raised focus:shadow-admin-focus rounded-admin-input py-admin-10 w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
                />
              </FilterField>
              <div className="gap-admin-9 grid grid-cols-2">
                <FilterField label="Finished after">
                  <input
                    type="date"
                    value={filters.finished_from ?? ''}
                    onChange={(event) =>
                      update('finished_from', event.target.value)
                    }
                    className="border-admin-border-input bg-admin-canvas text-admin-field text-admin-ink-strong placeholder:text-admin-muted-subtle focus:border-admin-accent focus:bg-admin-surface-raised focus:shadow-admin-focus rounded-admin-input py-admin-10 w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
                  />
                </FilterField>
                <FilterField label="Finished before">
                  <input
                    type="date"
                    value={filters.finished_to ?? ''}
                    onChange={(event) =>
                      update('finished_to', event.target.value)
                    }
                    className="border-admin-border-input bg-admin-canvas text-admin-field text-admin-ink-strong placeholder:text-admin-muted-subtle focus:border-admin-accent focus:bg-admin-surface-raised focus:shadow-admin-focus rounded-admin-input py-admin-10 w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
                  />
                </FilterField>
              </div>
            </div>

            <button
              type="button"
              onClick={clearFilters}
              disabled={activeFilters === 0}
              className="border-admin-accent/40 text-admin-accent disabled:border-admin-ink/10 rounded-admin-input p-admin-9 disabled:text-admin-muted-subtle text-admin-form mt-[1.15rem] w-full cursor-pointer border bg-transparent font-semibold disabled:cursor-not-allowed"
            >
              Clear all filters
            </button>
            <p className="border-admin-ink/9 text-admin-muted-subtle pt-admin-15 text-admin-note mt-4 mb-0 border-t leading-normal">
              Results are ordered by completion time for stable review and
              pagination.
            </p>
          </AdminPanel>
        </aside>

        <div className="min-w-0">
          <div className="mb-4 flex min-h-13.5 items-center justify-between gap-4 max-[700px]:flex-col max-[700px]:items-start">
            <div>
              <p className="text-admin-accent text-admin-note m-0 font-mono font-medium tracking-[0.13em] uppercase">
                Search results
              </p>
              <h2 className="text-admin-ink-strong mt-admin-4 text-admin-section mb-0 font-semibold">
                {loading
                  ? 'Searching archive...'
                  : `${pageStart}-${pageEnd} of ${total}`}
              </h2>
            </div>
            <Pagination
              offset={offset}
              pageSize={pageSize}
              itemCount={games.length}
              loading={loading}
              onOffsetChange={setOffset}
              label="Game result pages"
            />
          </div>

          {message ? <Notice variant="error">{message}</Notice> : null}
          {!loading && games.length === 0 ? (
            <EmptyState
              mark="7S"
              title="No games match this query"
              description="Remove a filter or verify the identifiers before searching again."
            />
          ) : null}
          <div className="grid gap-3">
            {games.map((game) => (
              <GameResult key={game.game_id} game={game} />
            ))}
          </div>
        </div>
      </div>
    </AdminPage>
  )
}

function GameResult({ game }: { game: Game }) {
  const finished = Boolean(game.finished_at)
  return (
    <Link
      to={`/games/${game.game_id}`}
      className="border-admin-ink/11 bg-admin-surface/84 hover:border-admin-accent/48 hover:bg-admin-surface-raised px-admin-17 py-admin-card-y relative grid grid-cols-[minmax(260px,1.15fr)_minmax(320px,1fr)_auto] items-center gap-5 overflow-hidden rounded-xl border text-inherit no-underline transition-[transform,border-color,background] duration-140 hover:-translate-y-0.5 max-[1180px]:grid-cols-[1fr_auto] max-[700px]:grid-cols-1"
    >
      <div className="gap-admin-15 flex min-w-0 items-center">
        <span
          className="border-admin-accent/35 bg-admin-ink rounded-admin-input text-admin-button-ink-dark grid h-12.5 w-10.5 shrink-0 place-items-center border font-mono text-base font-semibold shadow-[0_5px_14px_rgb(0_0_0/25%)]"
          aria-hidden="true"
        >
          S
        </span>
        <div>
          <div className="gap-admin-6 flex min-w-0 flex-wrap items-center">
            <h3 className="text-admin-ink-strong m-0 max-w-full overflow-hidden text-[0.95rem] font-semibold text-ellipsis whitespace-nowrap">
              {game.room_name || 'Unnamed room'}
            </h3>
            <span
              className={`gap-admin-4 py-admin-chip-y text-admin-session inline-flex items-center rounded-full border px-[0.55rem] font-mono tracking-[0.04em] uppercase before:size-1.25 before:rounded-full before:bg-current before:content-[''] ${finished ? 'border-admin-success-border bg-admin-success-bg text-admin-success' : 'border-admin-accent-border-subtle bg-admin-accent-soft text-admin-warning'}`}
            >
              {finished ? 'Completed' : 'Incomplete'}
            </span>
            {!game.replay_available ? (
              <span className="border-admin-accent-border-subtle bg-admin-accent-soft gap-admin-4 px-admin-7 text-admin-warning py-admin-chip-y text-admin-session inline-flex items-center rounded-full border font-mono tracking-[0.04em] uppercase before:size-1.25 before:rounded-full before:bg-current before:content-['']">
                Replay missing
              </span>
            ) : null}
          </div>
          <p className="text-admin-muted-subtle mt-admin-4 text-admin-caption mb-0 max-w-77.5 overflow-hidden font-mono text-ellipsis whitespace-nowrap">
            {game.game_id}
          </p>
        </div>
      </div>
      <dl className="max-[1180px]:border-admin-ink/8 gap-admin-13 max-[1180px]:pt-admin-10 m-0 grid grid-cols-[0.8fr_1fr_1.25fr] max-[1180px]:col-span-full max-[1180px]:row-start-2 max-[1180px]:border-t max-[700px]:col-auto max-[700px]:row-auto max-[700px]:grid-cols-2">
        <div>
          <dt className="text-admin-muted-subtle text-admin-micro mb-1 font-mono tracking-[0.06em] uppercase">
            Mode
          </dt>
          <dd className="text-admin-ink-soft text-admin-data m-0">
            {formatLabel(game.mode)}
          </dd>
        </div>
        <div>
          <dt className="text-admin-muted-subtle text-admin-micro mb-1 font-mono tracking-[0.06em] uppercase">
            Room
          </dt>
          <dd
            className="text-admin-ink-soft text-admin-data m-0"
            title={game.room_id}
          >
            {shortID(game.room_id)}
          </dd>
        </div>
        <div className="max-[700px]:col-span-full">
          <dt className="text-admin-muted-subtle text-admin-micro mb-1 font-mono tracking-[0.06em] uppercase">
            Finished
          </dt>
          <dd className="text-admin-ink-soft text-admin-data m-0">
            {game.finished_at
              ? formatDateTime(game.finished_at)
              : 'Not recorded'}
          </dd>
        </div>
      </dl>
      <span
        className="text-admin-accent text-admin-meta font-mono uppercase max-[1180px]:col-start-2 max-[1180px]:row-start-1"
        aria-hidden="true"
      >
        View
      </span>
    </Link>
  )
}
