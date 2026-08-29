import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { searchGames, type Game, type GameFilters } from '../api/games'
import { Notice } from './Feedback'
import { EmptyState, FilterField, Pagination } from './InvestigationUI'
import { formatDateTime, formatLabel, shortID } from './formatters'

const inputClass =
  'w-full min-w-0 rounded-[7px] border border-[#f4ead5]/15 bg-[#0d1a12] px-3 py-[0.7rem] text-[0.8rem] text-[#fafaf8] outline-none transition-[border-color,box-shadow,background] duration-[120ms] placeholder:text-[#5f665e] focus:border-[#c9922b] focus:bg-[#101f16] focus:shadow-[0_0_0_3px_rgb(201_146_43_/_14%)]'

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
    <section
      className="mx-auto w-full max-w-360"
      aria-labelledby="games-heading"
    >
      <header className="border-admin-ink/12 flex items-end justify-between gap-8 border-b pb-8 max-[700px]:flex-col max-[700px]:items-stretch">
        <div>
          <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase">
            Archive / Game investigations
          </p>
          <h1
            id="games-heading"
            className="text-admin-ink-strong mt-[0.55rem] mb-[0.65rem] text-[clamp(2.25rem,5vw,4.6rem)] leading-[0.98] font-medium tracking-[-0.055em]"
          >
            Games
          </h1>
          <p className="text-admin-muted m-0 max-w-170 text-[0.95rem] leading-[1.65]">
            Search recorded games, trace a result from room to final move, then
            preserve review context without altering the record.
          </p>
        </div>
        <div
          className="border-admin-accent/35 bg-admin-accent/7 min-w-41.25 rounded-xl border px-[1.2rem] py-4 text-right max-[700px]:min-w-0 max-[700px]:text-left"
          aria-label={`${total} games found`}
        >
          <strong className="text-admin-accent-bright block font-mono text-[1.8rem] font-medium">
            {total}
          </strong>
          <span className="text-admin-muted text-[0.72rem]">
            matching records
          </span>
        </div>
      </header>

      <div className="mt-8 grid grid-cols-[minmax(245px,310px)_minmax(0,1fr)] items-start gap-8 max-[980px]:grid-cols-1">
        <aside
          className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel sticky top-6 rounded-[14px] border p-5 max-[980px]:static"
          aria-labelledby="filter-heading"
        >
          <div className="flex items-center justify-between gap-4">
            <div>
              <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase">
                Query builder
              </p>
              <h2
                id="filter-heading"
                className="text-admin-ink-strong mt-[0.35rem] mb-0 text-[1.15rem] font-semibold"
              >
                Narrow the archive
              </h2>
            </div>
            {activeFilters > 0 ? (
              <span className="bg-admin-accent grid size-6.5 place-items-center rounded-full font-mono text-[0.7rem] font-medium text-[#1a1204]">
                {activeFilters}
              </span>
            ) : null}
          </div>

          <div className="mt-[1.4rem] grid gap-[0.9rem] max-[980px]:grid-cols-2 max-[700px]:grid-cols-1">
            <FilterField label="Game ID">
              <input
                value={filters.id ?? ''}
                onChange={(event) => update('id', event.target.value)}
                placeholder="UUID or exact ID"
                className={inputClass}
              />
            </FilterField>
            <FilterField label="Room ID">
              <input
                value={filters.room_id ?? ''}
                onChange={(event) => update('room_id', event.target.value)}
                placeholder="Room UUID"
                className={inputClass}
              />
            </FilterField>
            <FilterField label="Player ID">
              <input
                value={filters.player_id ?? ''}
                onChange={(event) => update('player_id', event.target.value)}
                placeholder="Player UUID"
                className={inputClass}
              />
            </FilterField>
            <div className="grid grid-cols-2 gap-[0.65rem]">
              <FilterField label="Mode">
                <input
                  value={filters.mode ?? ''}
                  onChange={(event) => update('mode', event.target.value)}
                  placeholder="classic"
                  className={inputClass}
                />
              </FilterField>
              <FilterField label="Completion">
                <select
                  value={filters.completion ?? ''}
                  onChange={(event) => update('completion', event.target.value)}
                  className={inputClass}
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
                className={inputClass}
              />
            </FilterField>
            <div className="grid grid-cols-2 gap-[0.65rem]">
              <FilterField label="Finished after">
                <input
                  type="date"
                  value={filters.finished_from ?? ''}
                  onChange={(event) =>
                    update('finished_from', event.target.value)
                  }
                  className={inputClass}
                />
              </FilterField>
              <FilterField label="Finished before">
                <input
                  type="date"
                  value={filters.finished_to ?? ''}
                  onChange={(event) =>
                    update('finished_to', event.target.value)
                  }
                  className={inputClass}
                />
              </FilterField>
            </div>
          </div>

          <button
            type="button"
            onClick={clearFilters}
            disabled={activeFilters === 0}
            className="border-admin-accent/40 text-admin-accent disabled:border-admin-ink/10 mt-[1.15rem] w-full cursor-pointer rounded-[7px] border bg-transparent p-[0.65rem] text-[0.76rem] font-semibold disabled:cursor-not-allowed disabled:text-[#5a5550]"
          >
            Clear all filters
          </button>
          <p className="border-admin-ink/9 text-admin-muted-subtle mt-4 mb-0 border-t pt-[0.9rem] text-[0.68rem] leading-normal">
            Results are ordered by completion time for stable review and
            pagination.
          </p>
        </aside>

        <div className="min-w-0">
          <div className="mb-4 flex min-h-13.5 items-center justify-between gap-4 max-[700px]:flex-col max-[700px]:items-start">
            <div>
              <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase">
                Search results
              </p>
              <h2 className="text-admin-ink-strong mt-[0.35rem] mb-0 text-[1.15rem] font-semibold">
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
    </section>
  )
}

function GameResult({ game }: { game: Game }) {
  const finished = Boolean(game.finished_at)
  return (
    <Link
      to={`/games/${game.game_id}`}
      className="border-admin-ink/11 bg-admin-surface/84 hover:border-admin-accent/48 hover:bg-admin-surface-raised relative grid grid-cols-[minmax(260px,1.15fr)_minmax(320px,1fr)_auto] items-center gap-5 overflow-hidden rounded-xl border px-[1.2rem] py-[1.05rem] text-inherit no-underline transition-[transform,border-color,background] duration-140 hover:-translate-y-0.5 max-[1180px]:grid-cols-[1fr_auto] max-[700px]:grid-cols-1"
    >
      <div className="flex min-w-0 items-center gap-[0.9rem]">
        <span
          className="border-admin-accent/35 bg-admin-ink grid h-12.5 w-10.5 shrink-0 place-items-center rounded-[7px] border font-mono text-base font-semibold text-[#1a1a1a] shadow-[0_5px_14px_rgb(0_0_0/25%)]"
          aria-hidden="true"
        >
          S
        </span>
        <div>
          <div className="flex min-w-0 flex-wrap items-center gap-[0.45rem]">
            <h3 className="text-admin-ink-strong m-0 max-w-full overflow-hidden text-[0.95rem] font-semibold text-ellipsis whitespace-nowrap">
              {game.room_name || 'Unnamed room'}
            </h3>
            <span
              className={`inline-flex items-center gap-[0.35rem] rounded-full border px-[0.55rem] py-[0.22rem] font-mono text-[0.58rem] tracking-[0.04em] uppercase before:size-1.25 before:rounded-full before:bg-current before:content-[''] ${finished ? 'text-admin-success border-[#2d7a46]/60 bg-[#2d7a46]/17' : 'border-admin-accent/45 bg-admin-accent/10 text-[#e0b45e]'}`}
            >
              {finished ? 'Completed' : 'Incomplete'}
            </span>
            {!game.replay_available ? (
              <span className="border-admin-accent/45 bg-admin-accent/10 inline-flex items-center gap-[0.35rem] rounded-full border px-[0.55rem] py-[0.22rem] font-mono text-[0.58rem] tracking-[0.04em] text-[#e0b45e] uppercase before:size-1.25 before:rounded-full before:bg-current before:content-['']">
                Replay missing
              </span>
            ) : null}
          </div>
          <p className="text-admin-muted-subtle mt-[0.35rem] mb-0 max-w-77.5 overflow-hidden font-mono text-[0.62rem] text-ellipsis whitespace-nowrap">
            {game.game_id}
          </p>
        </div>
      </div>
      <dl className="max-[1180px]:border-admin-ink/8 m-0 grid grid-cols-[0.8fr_1fr_1.25fr] gap-[0.8rem] max-[1180px]:col-span-full max-[1180px]:row-start-2 max-[1180px]:border-t max-[1180px]:pt-[0.7rem] max-[700px]:col-auto max-[700px]:row-auto max-[700px]:grid-cols-2">
        <div>
          <dt className="text-admin-muted-subtle mb-1 font-mono text-[0.57rem] tracking-[0.06em] uppercase">
            Mode
          </dt>
          <dd className="m-0 text-[0.73rem] text-[#d9d4c8]">
            {formatLabel(game.mode)}
          </dd>
        </div>
        <div>
          <dt className="text-admin-muted-subtle mb-1 font-mono text-[0.57rem] tracking-[0.06em] uppercase">
            Room
          </dt>
          <dd
            className="m-0 text-[0.73rem] text-[#d9d4c8]"
            title={game.room_id}
          >
            {shortID(game.room_id)}
          </dd>
        </div>
        <div className="max-[700px]:col-span-full">
          <dt className="text-admin-muted-subtle mb-1 font-mono text-[0.57rem] tracking-[0.06em] uppercase">
            Finished
          </dt>
          <dd className="m-0 text-[0.73rem] text-[#d9d4c8]">
            {game.finished_at
              ? formatDateTime(game.finished_at)
              : 'Not recorded'}
          </dd>
        </div>
      </dl>
      <span
        className="text-admin-accent font-mono text-[0.65rem] uppercase max-[1180px]:col-start-2 max-[1180px]:row-start-1"
        aria-hidden="true"
      >
        View
      </span>
    </Link>
  )
}
