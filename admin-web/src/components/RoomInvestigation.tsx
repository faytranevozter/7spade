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

const inputClass =
  'w-full min-w-0 rounded-[7px] border border-[#f4ead5]/15 bg-[#0d1a12] px-3 py-[0.7rem] text-[0.8rem] text-[#fafaf8] outline-none transition-[border-color,box-shadow,background] duration-[120ms] placeholder:text-[#5f665e] focus:border-[#c9922b] focus:bg-[#101f16] focus:shadow-[0_0_0_3px_rgb(201_146_43_/_14%)]'

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
          <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase">
            Operations / Room investigations
          </p>
          <h1
            id="rooms-heading"
            className="text-admin-ink-strong mt-[0.55rem] mb-[0.65rem] text-[clamp(2.25rem,5vw,4.6rem)] leading-[0.98] font-medium tracking-[-0.055em]"
          >
            Rooms
          </h1>
          <p className="text-admin-muted m-0 max-w-175 text-[0.95rem] leading-[1.65]">
            Locate a durable room record, verify its configuration, and inspect
            safely redacted live authority when available.
          </p>
        </div>
        <div
          className="border-admin-ink/11 bg-admin-surface/80 [&_span]:text-admin-muted-subtle [&_strong]:text-admin-accent-bright [&>div]:border-admin-ink/9 grid min-w-82.5 grid-cols-3 overflow-hidden rounded-xl border max-[760px]:min-w-0 max-[480px]:grid-cols-1 [&_span]:mt-[0.18rem] [&_span]:block [&_span]:text-[0.6rem] [&_strong]:block [&_strong]:font-mono [&_strong]:text-[1.25rem] [&_strong]:font-medium [&>div]:border-r [&>div]:p-[0.85rem] max-[480px]:[&>div]:border-r-0 max-[480px]:[&>div]:border-b [&>div:last-child]:border-r-0 max-[480px]:[&>div:last-child]:border-b-0"
          aria-label="Room result summary"
        >
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

      <div className="mt-8 grid grid-cols-[minmax(245px,310px)_minmax(0,1fr)] items-start gap-8 max-[1000px]:grid-cols-1">
        <aside
          className="border-admin-ink/11 bg-admin-surface/88 shadow-admin-panel sticky top-6 rounded-[14px] border p-5 max-[1000px]:static"
          aria-labelledby="room-filter-heading"
        >
          <div className="[&_h2]:text-admin-ink-strong flex items-center justify-between gap-4 [&_h2]:mt-[0.35rem] [&_h2]:mb-0 [&_h2]:text-[1.15rem] [&_h2]:font-semibold">
            <div>
              <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase">
                Query builder
              </p>
              <h2 id="room-filter-heading">Find a room</h2>
            </div>
            {activeFilters ? (
              <span className="bg-admin-accent grid size-6.5 place-items-center rounded-full font-mono text-[0.7rem] font-medium text-[#1a1204]">
                {activeFilters}
              </span>
            ) : null}
          </div>
          <div className="mt-[1.4rem] grid gap-[0.9rem] max-[1000px]:grid-cols-2 max-[760px]:grid-cols-1">
            <FilterField label="Room ID">
              <input
                value={filters.id ?? ''}
                onChange={(event) => update('id', event.target.value)}
                placeholder="Exact room UUID"
                className={inputClass}
              />
            </FilterField>
            <FilterField label="Invite code">
              <input
                value={filters.invite_code ?? ''}
                onChange={(event) => update('invite_code', event.target.value)}
                placeholder="e.g. ACE123"
                className={`${inputClass} font-mono tracking-[0.08em] uppercase`}
              />
            </FilterField>
            <div className="grid grid-cols-2 gap-[0.65rem]">
              <FilterField label="Status">
                <select
                  value={filters.status ?? ''}
                  onChange={(event) => update('status', event.target.value)}
                  className={inputClass}
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
                  className={inputClass}
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
                className={inputClass}
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
                className={inputClass}
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
                className={inputClass}
              />
            </FilterField>
          </div>
          <button
            type="button"
            onClick={clearFilters}
            disabled={!activeFilters}
            className="border-admin-accent/40 text-admin-accent disabled:border-admin-ink/10 mt-[1.15rem] w-full cursor-pointer rounded-[7px] border bg-transparent p-[0.65rem] text-[0.76rem] font-semibold disabled:cursor-not-allowed disabled:text-[#5a5550]"
          >
            Clear all filters
          </button>
          <p className="border-admin-ink/9 text-admin-muted-subtle mt-4 mb-0 border-t pt-[0.9rem] text-[0.68rem] leading-normal">
            Durable records are ordered by creation time. Opening one may also
            request a redacted snapshot from the current WS owner.
          </p>
        </aside>

        <div className="min-w-0">
          <div className="[&_h2]:text-admin-ink-strong mb-4 flex min-h-13.5 items-center justify-between gap-4 max-[760px]:flex-col max-[760px]:items-start [&_h2]:mt-[0.35rem] [&_h2]:mb-0 [&_h2]:text-[1.15rem] [&_h2]:font-semibold">
            <div>
              <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase">
                Durable records
              </p>
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
      className="border-admin-ink/11 bg-admin-surface/84 hover:border-admin-accent/48 hover:bg-admin-surface-raised relative grid grid-cols-[minmax(270px,1.15fr)_105px_minmax(330px,1fr)_auto] items-center gap-[1.2rem] overflow-hidden rounded-xl border px-[1.2rem] py-[1.05rem] text-inherit no-underline transition-[transform,border-color,background] duration-140 hover:-translate-y-0.5 max-[1220px]:grid-cols-[minmax(250px,1fr)_100px_auto] max-[760px]:grid-cols-1"
    >
      <div className="[&_p]:text-admin-muted-subtle flex min-w-0 items-center gap-[0.8rem] [&_p]:mt-[0.35rem] [&_p]:mb-0 [&_p]:max-w-75 [&_p]:overflow-hidden [&_p]:font-mono [&_p]:text-[0.6rem] [&_p]:text-ellipsis [&_p]:whitespace-nowrap">
        <span
          className={`bg-admin-muted-subtle size-2.5 shrink-0 rounded-full shadow-[0_0_0_5px_rgb(119_119_111/10%)] ${room.status === 'in_progress' ? 'bg-[#56b875] shadow-[0_0_0_5px_rgb(45_122_70/18%)]' : room.status === 'waiting' ? 'bg-admin-accent shadow-[0_0_0_5px_rgb(201_146_43/14%)]' : ''}`}
          aria-hidden="true"
        />
        <div>
          <div className="[&_h3]:text-admin-ink-strong flex min-w-0 flex-wrap items-center gap-[0.45rem] [&_h3]:m-0 [&_h3]:max-w-full [&_h3]:overflow-hidden [&_h3]:text-[0.95rem] [&_h3]:font-semibold [&_h3]:text-ellipsis [&_h3]:whitespace-nowrap">
            <h3>{room.name || `Room ${room.invite_code}`}</h3>
            <RoomStatus value={room.status} />
            <span className="text-admin-muted inline-flex items-center rounded-full bg-white/4 px-[0.55rem] py-[0.22rem] font-mono text-[0.56rem] tracking-[0.04em] uppercase">
              {room.visibility}
            </span>
          </div>
          <p>{room.id}</p>
        </div>
      </div>
      <div
        className="[&_small]:text-admin-muted-subtle [&_span]:text-admin-muted-subtle [&_strong]:text-admin-ink max-[760px]:max-w-37.5 [&_small]:text-[0.62rem] [&_span]:text-[0.6rem] [&_strong]:font-mono [&_strong]:text-[0.8rem] [&>div:first-child]:flex [&>div:first-child]:items-baseline [&>div:first-child]:justify-between"
        aria-label={`${room.player_count} of ${room.max_players} seats occupied`}
      >
        <div>
          <span>Seats</span>
          <strong>
            {room.player_count}
            <small> / {room.max_players}</small>
          </strong>
        </div>
        <div className="bg-admin-ink/10 [&_span]:bg-admin-accent mt-[0.45rem] h-0.75 overflow-hidden rounded-[99px] [&_span]:block [&_span]:h-full [&_span]:rounded-[inherit]">
          <span style={{ width: `${occupancy}%` }} />
        </div>
      </div>
      <dl className="max-[1220px]:border-admin-ink/8 [&_dt]:text-admin-muted-subtle m-0 grid grid-cols-[0.8fr_0.8fr_1fr_1.3fr] gap-[0.7rem] max-[1220px]:col-span-full max-[1220px]:row-start-2 max-[1220px]:border-t max-[1220px]:pt-[0.7rem] max-[760px]:col-auto max-[760px]:row-auto max-[760px]:grid-cols-2 [&_dd]:m-0 [&_dd]:overflow-hidden [&_dd]:text-[0.7rem] [&_dd]:text-ellipsis [&_dd]:whitespace-nowrap [&_dd]:text-[#d9d4c8] [&_dt]:mb-[0.22rem] [&_dt]:font-mono [&_dt]:text-[0.55rem] [&_dt]:uppercase [&>div:first-child_dd]:font-mono [&>div:first-child_dd]:tracking-[0.06em] [&>div:first-child_dd]:text-[#e0b45e]">
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
      <span
        className="text-admin-accent font-mono text-[0.65rem] uppercase max-[1220px]:col-start-3 max-[1220px]:row-start-1 max-[760px]:hidden"
        aria-hidden="true"
      >
        Inspect
      </span>
    </Link>
  )
}
