import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { searchGames, type Game, type GameFilters } from '../api/games'
import { Notice } from './Feedback'
import { EmptyState, FilterField, Pagination } from './InvestigationUI'
import { formatDateTime, formatLabel, shortID } from './formatters'

const inputClass = 'game-input'

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
    <section className="investigation-page" aria-labelledby="games-heading">
      <header className="investigation-header">
        <div>
          <p className="eyebrow">Archive / Game investigations</p>
          <h1 id="games-heading">Games</h1>
          <p>
            Search recorded games, trace a result from room to final move, then
            preserve review context without altering the record.
          </p>
        </div>
        <div className="header-stat" aria-label={`${total} games found`}>
          <strong>{total}</strong>
          <span>matching records</span>
        </div>
      </header>

      <div className="investigation-layout">
        <aside className="filter-panel" aria-labelledby="filter-heading">
          <div className="panel-heading">
            <div>
              <p className="eyebrow">Query builder</p>
              <h2 id="filter-heading">Narrow the archive</h2>
            </div>
            {activeFilters > 0 ? (
              <span className="filter-count">{activeFilters}</span>
            ) : null}
          </div>

          <div className="filter-fields">
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
            <div className="filter-row">
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
            <div className="filter-row">
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
            className="clear-filter-button"
          >
            Clear all filters
          </button>
          <p className="filter-help">
            Results are ordered by completion time for stable review and
            pagination.
          </p>
        </aside>

        <div className="results-panel">
          <div className="results-toolbar">
            <div>
              <p className="eyebrow">Search results</p>
              <h2>
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
          <div className="game-results">
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
    <Link to={`/games/${game.game_id}`} className="game-result-card">
      <div className="game-result-primary">
        <span className="game-spade" aria-hidden="true">
          S
        </span>
        <div>
          <div className="result-title-row">
            <h3>{game.room_name || 'Unnamed room'}</h3>
            <span
              className={`status-pill ${finished ? 'status-complete' : 'status-incomplete'}`}
            >
              {finished ? 'Completed' : 'Incomplete'}
            </span>
            {!game.replay_available ? (
              <span className="status-pill status-warning">Replay missing</span>
            ) : null}
          </div>
          <p className="mono-id">{game.game_id}</p>
        </div>
      </div>
      <dl className="game-result-meta">
        <div>
          <dt>Mode</dt>
          <dd>{formatLabel(game.mode)}</dd>
        </div>
        <div>
          <dt>Room</dt>
          <dd title={game.room_id}>{shortID(game.room_id)}</dd>
        </div>
        <div>
          <dt>Finished</dt>
          <dd>
            {game.finished_at
              ? formatDateTime(game.finished_at)
              : 'Not recorded'}
          </dd>
        </div>
      </dl>
      <span className="result-arrow" aria-hidden="true">
        View
      </span>
    </Link>
  )
}
