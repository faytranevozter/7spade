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
      className="mx-auto w-full max-w-360"
      aria-labelledby="rooms-heading"
    >
      <header className="border-admin-ink/12 flex items-end justify-between gap-8 border-b pb-8 max-[760px]:flex-col max-[760px]:items-stretch">
        <div>
          <p className="text-admin-accent text-admin-note m-0 font-mono font-medium tracking-[0.13em] uppercase">
            Operations / Room investigations
          </p>
          <h1
            id="rooms-heading"
            className="text-admin-ink-strong mt-admin-7 mb-admin-9 text-admin-investigation-hero font-medium"
          >
            Rooms
          </h1>
          <p className="text-admin-muted m-0 max-w-175 text-[0.95rem] leading-[1.65]">
            Locate a durable room record, verify its configuration, and inspect
            safely redacted live authority when available.
          </p>
        </div>
        <div
          className="border-admin-ink/11 bg-admin-surface/80 grid min-w-82.5 grid-cols-3 overflow-hidden rounded-xl border max-[760px]:min-w-0 max-[480px]:grid-cols-1"
          aria-label="Room result summary"
        >
          <div className="border-admin-ink/9 p-admin-14 border-r max-[480px]:border-r-0 max-[480px]:border-b">
            <strong className="text-admin-accent-bright text-admin-metric block font-mono font-medium">
              {rooms.length}
            </strong>
            <span className="text-admin-muted-subtle text-admin-label mt-admin-badge-y block">
              on this page
            </span>
          </div>
          <div className="border-admin-ink/9 p-admin-14 border-r max-[480px]:border-r-0 max-[480px]:border-b">
            <strong className="text-admin-accent-bright text-admin-metric block font-mono font-medium">
              {activeRooms}
            </strong>
            <span className="text-admin-muted-subtle text-admin-label mt-admin-badge-y block">
              in progress
            </span>
          </div>
          <div className="p-admin-14 max-[480px]:border-b-0">
            <strong className="text-admin-accent-bright text-admin-metric block font-mono font-medium">
              {occupiedSeats}
            </strong>
            <span className="text-admin-muted-subtle text-admin-label mt-admin-badge-y block">
              occupied seats
            </span>
          </div>
        </div>
      </header>

      <div className="mt-8 grid grid-cols-[minmax(245px,310px)_minmax(0,1fr)] items-start gap-8 max-[1000px]:grid-cols-1">
        <aside
          className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel rounded-admin-panel sticky top-6 border p-5 max-[1000px]:static"
          aria-labelledby="room-filter-heading"
        >
          <div className="flex items-center justify-between gap-4">
            <div>
              <p className="text-admin-accent text-admin-note m-0 font-mono font-medium tracking-[0.13em] uppercase">
                Query builder
              </p>
              <h2
                id="room-filter-heading"
                className="text-admin-ink-strong mt-admin-4 text-admin-section mb-0 font-semibold"
              >
                Find a room
              </h2>
            </div>
            {activeFilters ? (
              <span className="bg-admin-accent text-admin-button-ink text-admin-small grid size-6.5 place-items-center rounded-full font-mono font-medium">
                {activeFilters}
              </span>
            ) : null}
          </div>
          <div className="mt-admin-18 gap-admin-15 grid max-[1000px]:grid-cols-2 max-[760px]:grid-cols-1">
            <FilterField label="Room ID">
              <input
                value={filters.id ?? ''}
                onChange={(event) => update('id', event.target.value)}
                placeholder="Exact room UUID"
                className="border-admin-border-input bg-admin-canvas text-admin-field text-admin-ink-strong placeholder:text-admin-muted-subtle focus:border-admin-accent focus:bg-admin-surface-raised focus:shadow-admin-focus rounded-admin-input py-admin-10 w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
              />
            </FilterField>
            <FilterField label="Invite code">
              <input
                value={filters.invite_code ?? ''}
                onChange={(event) => update('invite_code', event.target.value)}
                placeholder="e.g. ACE123"
                className="border-admin-border-input bg-admin-canvas text-admin-field text-admin-ink-strong placeholder:text-admin-muted-subtle focus:border-admin-accent focus:bg-admin-surface-raised focus:shadow-admin-focus rounded-admin-input py-admin-10 w-full min-w-0 border px-3 font-mono tracking-[0.08em] uppercase transition-[border-color,box-shadow,background] duration-120 outline-none"
              />
            </FilterField>
            <div className="gap-admin-9 grid grid-cols-2">
              <FilterField label="Status">
                <select
                  value={filters.status ?? ''}
                  onChange={(event) => update('status', event.target.value)}
                  className="border-admin-border-input bg-admin-canvas text-admin-field text-admin-ink-strong placeholder:text-admin-muted-subtle focus:border-admin-accent focus:bg-admin-surface-raised focus:shadow-admin-focus rounded-admin-input py-admin-10 w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
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
                  className="border-admin-border-input bg-admin-canvas text-admin-field text-admin-ink-strong placeholder:text-admin-muted-subtle focus:border-admin-accent focus:bg-admin-surface-raised focus:shadow-admin-focus rounded-admin-input py-admin-10 w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
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
                className="border-admin-border-input bg-admin-canvas text-admin-field text-admin-ink-strong placeholder:text-admin-muted-subtle focus:border-admin-accent focus:bg-admin-surface-raised focus:shadow-admin-focus rounded-admin-input py-admin-10 w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
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
                className="border-admin-border-input bg-admin-canvas text-admin-field text-admin-ink-strong placeholder:text-admin-muted-subtle focus:border-admin-accent focus:bg-admin-surface-raised focus:shadow-admin-focus rounded-admin-input py-admin-10 w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
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
                className="border-admin-border-input bg-admin-canvas text-admin-field text-admin-ink-strong placeholder:text-admin-muted-subtle focus:border-admin-accent focus:bg-admin-surface-raised focus:shadow-admin-focus rounded-admin-input py-admin-10 w-full min-w-0 border px-3 transition-[border-color,box-shadow,background] duration-120 outline-none"
              />
            </FilterField>
          </div>
          <button
            type="button"
            onClick={clearFilters}
            disabled={!activeFilters}
            className="border-admin-accent/40 text-admin-accent disabled:border-admin-ink/10 rounded-admin-input p-admin-9 disabled:text-admin-muted-subtle text-admin-form mt-[1.15rem] w-full cursor-pointer border bg-transparent font-semibold disabled:cursor-not-allowed"
          >
            Clear all filters
          </button>
          <p className="border-admin-ink/9 text-admin-muted-subtle pt-admin-15 text-admin-note mt-4 mb-0 border-t leading-normal">
            Durable records are ordered by creation time. Opening one may also
            request a redacted snapshot from the current WS owner.
          </p>
        </aside>

        <div className="min-w-0">
          <div className="mb-4 flex min-h-13.5 items-center justify-between gap-4 max-[760px]:flex-col max-[760px]:items-start">
            <div>
              <p className="text-admin-accent text-admin-note m-0 font-mono font-medium tracking-[0.13em] uppercase">
                Durable records
              </p>
              <h2 className="text-admin-ink-strong mt-admin-4 text-admin-section mb-0 font-semibold">
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
              markClassName="h-[54px] w-[54px] rounded-full"
            />
          ) : null}
          <div className="grid gap-3">
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
    <Link
      to={`/rooms/${room.id}`}
      className="border-admin-ink/11 bg-admin-surface/84 hover:border-admin-accent/48 hover:bg-admin-surface-raised gap-admin-17 px-admin-17 py-admin-card-y relative grid grid-cols-[minmax(270px,1.15fr)_105px_minmax(330px,1fr)_auto] items-center overflow-hidden rounded-xl border text-inherit no-underline transition-[transform,border-color,background] duration-140 hover:-translate-y-0.5 max-[1220px]:grid-cols-[minmax(250px,1fr)_100px_auto] max-[760px]:grid-cols-1"
    >
      <div className="gap-admin-13 flex min-w-0 items-center">
        <span
          className={`bg-admin-muted-subtle size-2.5 shrink-0 rounded-full shadow-[0_0_0_5px_rgb(119_119_111/10%)] ${room.status === 'in_progress' ? 'bg-admin-success shadow-[0_0_0_5px_rgb(45_122_70/18%)]' : room.status === 'waiting' ? 'bg-admin-accent shadow-[0_0_0_5px_rgb(201_146_43/14%)]' : ''}`}
          aria-hidden="true"
        />
        <div>
          <div className="gap-admin-6 flex min-w-0 flex-wrap items-center">
            <h3 className="text-admin-ink-strong m-0 max-w-full overflow-hidden text-[0.95rem] font-semibold text-ellipsis whitespace-nowrap">
              {room.name || `Room ${room.invite_code}`}
            </h3>
            <RoomStatus value={room.status} />
            <span className="text-admin-muted px-admin-7 py-admin-chip-y text-admin-tiny inline-flex items-center rounded-full bg-white/4 font-mono tracking-[0.04em] uppercase">
              {room.visibility}
            </span>
          </div>
          <p className="text-admin-muted-subtle mt-admin-4 text-admin-label mb-0 max-w-75 overflow-hidden font-mono text-ellipsis whitespace-nowrap">
            {room.id}
          </p>
        </div>
      </div>
      <div
        className="max-[760px]:max-w-37.5"
        aria-label={`${room.player_count} of ${room.max_players} seats occupied`}
      >
        <div className="flex items-baseline justify-between">
          <span className="text-admin-muted-subtle text-admin-label">
            Seats
          </span>
          <strong className="text-admin-ink text-admin-preview font-mono">
            {room.player_count}
            <small className="text-admin-muted-subtle text-admin-caption">
              {' '}
              / {room.max_players}
            </small>
          </strong>
        </div>
        <div className="bg-admin-ink/10 mt-admin-6 h-0.75 overflow-hidden rounded-[99px]">
          <span
            className="bg-admin-accent block h-full rounded-[inherit]"
            style={{ width: `${occupancy}%` }}
          />
        </div>
      </div>
      <dl className="max-[1220px]:border-admin-ink/8 gap-admin-10 max-[1220px]:pt-admin-10 m-0 grid grid-cols-[0.8fr_0.8fr_1fr_1.3fr] max-[1220px]:col-span-full max-[1220px]:row-start-2 max-[1220px]:border-t max-[760px]:col-auto max-[760px]:row-auto max-[760px]:grid-cols-2">
        <div>
          <dt className="text-admin-muted-subtle text-admin-xs mb-admin-chip-y font-mono uppercase">
            Invite
          </dt>
          <dd className="text-admin-warning text-admin-small m-0 overflow-hidden font-mono tracking-[0.06em] text-ellipsis whitespace-nowrap">
            {room.invite_code}
          </dd>
        </div>
        <div>
          <dt className="text-admin-muted-subtle text-admin-xs mb-admin-chip-y font-mono uppercase">
            Mode
          </dt>
          <dd className="text-admin-ink-soft text-admin-small m-0 overflow-hidden text-ellipsis whitespace-nowrap">
            {formatLabel(room.game_mode)}
          </dd>
        </div>
        <div>
          <dt className="text-admin-muted-subtle text-admin-xs mb-admin-chip-y font-mono uppercase">
            Rules
          </dt>
          <dd className="text-admin-ink-soft text-admin-small m-0 overflow-hidden text-ellipsis whitespace-nowrap">
            {room.practice_mode ? 'Practice' : formatLabel(room.team_mode)}
          </dd>
        </div>
        <div>
          <dt className="text-admin-muted-subtle text-admin-xs mb-admin-chip-y font-mono uppercase">
            Created
          </dt>
          <dd className="text-admin-ink-soft text-admin-small m-0 overflow-hidden text-ellipsis whitespace-nowrap">
            {formatDateTime(room.created_at)}
          </dd>
        </div>
      </dl>
      <span
        className="text-admin-accent text-admin-meta font-mono uppercase max-[1220px]:col-start-3 max-[1220px]:row-start-1 max-[760px]:hidden"
        aria-hidden="true"
      >
        Inspect
      </span>
    </Link>
  )
}
