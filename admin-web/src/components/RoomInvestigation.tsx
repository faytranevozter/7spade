import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { searchRooms, type Room, type RoomFilters } from '../api/rooms'
import { Notice } from './Feedback'
import {
  EmptyState,
  FilterField,
  Pagination,
  RoomStatus,
} from './InvestigationUI'
import { formatDateTime, formatLabel, toLocalDateTime } from './formatters'

export function RoomInvestigation({ token }: { token: string }) {
  const [filters, setFilters] = useState<RoomFilters>({})
  const [rooms, setRooms] = useState<Room[]>([])
  const [message, setMessage] = useState('')
  const [loading, setLoading] = useState(true)
  const [offset, setOffset] = useState(0)
  const pageSize = 50

  useEffect(() => {
    let cancelled = false
    const request = window.setTimeout(() => {
      setLoading(true)
      searchRooms(token, filters, pageSize, offset)
        .then((page) => {
          if (!cancelled) {
            setRooms(page.rooms ?? [])
            setMessage('')
          }
        })
        .catch((error: unknown) => {
          if (!cancelled)
            setMessage(
              error instanceof Error ? error.message : 'Failed to search rooms',
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

  function update(key: keyof RoomFilters, value: string) {
    setFilters((current) => ({ ...current, [key]: value }))
    setOffset(0)
  }

  function clearFilters() {
    setFilters({})
    setOffset(0)
  }

  const activeFilters = Object.values(filters).filter(Boolean).length
  const occupiedSeats = rooms.reduce((sum, room) => sum + room.player_count, 0)
  const activeRooms = rooms.filter(
    (room) => room.status === 'in_progress',
  ).length

  return (
    <section
      className="room-investigation-page"
      aria-labelledby="rooms-heading"
    >
      <header className="room-investigation-header">
        <div>
          <p className="eyebrow">Operations / Room investigations</p>
          <h1 id="rooms-heading">Rooms</h1>
          <p>
            Locate a durable room record, verify its configuration, and inspect
            safely redacted live authority when available.
          </p>
        </div>
        <div className="room-header-stats" aria-label="Room result summary">
          <div>
            <strong>{rooms.length}</strong>
            <span>on this page</span>
          </div>
          <div>
            <strong>{activeRooms}</strong>
            <span>in progress</span>
          </div>
          <div>
            <strong>{occupiedSeats}</strong>
            <span>occupied seats</span>
          </div>
        </div>
      </header>

      <div className="room-investigation-layout">
        <aside
          className="room-filter-panel"
          aria-labelledby="room-filter-heading"
        >
          <div className="panel-heading">
            <div>
              <p className="eyebrow">Query builder</p>
              <h2 id="room-filter-heading">Find a room</h2>
            </div>
            {activeFilters ? (
              <span className="filter-count">{activeFilters}</span>
            ) : null}
          </div>
          <div className="room-filter-fields">
            <FilterField label="Room ID">
              <input
                value={filters.id ?? ''}
                onChange={(event) => update('id', event.target.value)}
                placeholder="Exact room UUID"
                className="game-input"
              />
            </FilterField>
            <FilterField label="Invite code">
              <input
                value={filters.invite_code ?? ''}
                onChange={(event) => update('invite_code', event.target.value)}
                placeholder="e.g. ACE123"
                className="game-input room-code-input"
              />
            </FilterField>
            <div className="filter-row">
              <FilterField label="Status">
                <select
                  value={filters.status ?? ''}
                  onChange={(event) => update('status', event.target.value)}
                  className="game-input"
                >
                  <option value="">Any status</option>
                  <option value="waiting">Waiting</option>
                  <option value="in_progress">In progress</option>
                  <option value="finished">Finished</option>
                </select>
              </FilterField>
              <FilterField label="Visibility">
                <select
                  value={filters.visibility ?? ''}
                  onChange={(event) => update('visibility', event.target.value)}
                  className="game-input"
                >
                  <option value="">Any visibility</option>
                  <option value="public">Public</option>
                  <option value="private">Private</option>
                </select>
              </FilterField>
            </div>
            <FilterField label="Mode">
              <input
                value={filters.mode ?? ''}
                onChange={(event) => update('mode', event.target.value)}
                placeholder="classic or custom"
                className="game-input"
              />
            </FilterField>
            <FilterField label="Created after">
              <input
                type="datetime-local"
                value={toLocalDateTime(filters.created_from)}
                onChange={(event) =>
                  update(
                    'created_from',
                    event.target.value
                      ? new Date(event.target.value).toISOString()
                      : '',
                  )
                }
                className="game-input"
              />
            </FilterField>
            <FilterField label="Created before">
              <input
                type="datetime-local"
                value={toLocalDateTime(filters.created_to)}
                onChange={(event) =>
                  update(
                    'created_to',
                    event.target.value
                      ? new Date(event.target.value).toISOString()
                      : '',
                  )
                }
                className="game-input"
              />
            </FilterField>
          </div>
          <button
            type="button"
            onClick={clearFilters}
            disabled={!activeFilters}
            className="clear-filter-button"
          >
            Clear all filters
          </button>
          <p className="filter-help">
            Durable records are ordered by creation time. Opening one may also
            request a redacted snapshot from the current WS owner.
          </p>
        </aside>

        <div className="room-results-panel">
          <div className="results-toolbar">
            <div>
              <p className="eyebrow">Durable records</p>
              <h2>
                {loading
                  ? 'Searching rooms...'
                  : `${rooms.length} results on page ${Math.floor(offset / pageSize) + 1}`}
              </h2>
            </div>
            <Pagination
              offset={offset}
              pageSize={pageSize}
              itemCount={rooms.length}
              loading={loading}
              onOffsetChange={setOffset}
              label="Room result pages"
            />
          </div>
          {message ? <Notice variant="error">{message}</Notice> : null}
          {!loading && rooms.length === 0 ? (
            <EmptyState
              mark="R"
              title="No rooms found"
              description="Try a broader date window or remove an identifier filter."
              className="room-empty-state"
            />
          ) : null}
          <div className="room-results">
            {rooms.map((room) => (
              <RoomResult key={room.id} room={room} />
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}

function RoomResult({ room }: { room: Room }) {
  const occupancy = room.max_players
    ? Math.min(100, Math.round((room.player_count / room.max_players) * 100))
    : 0
  return (
    <Link to={`/rooms/${room.id}`} className="room-result-card">
      <div className="room-result-identity">
        <span
          className={`room-state-mark room-state-${room.status}`}
          aria-hidden="true"
        />
        <div>
          <div className="room-title-row">
            <h3>{room.name || `Room ${room.invite_code}`}</h3>
            <RoomStatus value={room.status} />
            <span className="room-visibility">{room.visibility}</span>
          </div>
          <p>{room.id}</p>
        </div>
      </div>
      <div
        className="room-seat-meter"
        aria-label={`${room.player_count} of ${room.max_players} seats occupied`}
      >
        <div>
          <span>Seats</span>
          <strong>
            {room.player_count}
            <small> / {room.max_players}</small>
          </strong>
        </div>
        <div className="seat-track">
          <span style={{ width: `${occupancy}%` }} />
        </div>
      </div>
      <dl className="room-result-meta">
        <div>
          <dt>Invite</dt>
          <dd>{room.invite_code}</dd>
        </div>
        <div>
          <dt>Mode</dt>
          <dd>{formatLabel(room.game_mode)}</dd>
        </div>
        <div>
          <dt>Rules</dt>
          <dd>
            {room.practice_mode ? 'Practice' : formatLabel(room.team_mode)}
          </dd>
        </div>
        <div>
          <dt>Created</dt>
          <dd>{formatDateTime(room.created_at)}</dd>
        </div>
      </dl>
      <span className="result-arrow" aria-hidden="true">
        Inspect
      </span>
    </Link>
  )
}
