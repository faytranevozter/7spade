import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import {
  getRoom,
  type LiveRoomSummary,
  type RoomDetail,
  type RoomPlayer,
} from '../api/rooms'
import { Notice } from '../components/Feedback'
import {
  RoomStatus,
  SectionHeading,
  SummaryItem,
} from '../components/InvestigationUI'
import {
  formatDateTime,
  formatLabel,
  formatTime,
  shortID,
} from '../components/formatters'
import { useAuth } from '../hooks/useAuth'

export function RoomDetailPage() {
  const { id = '' } = useParams()
  const { token } = useAuth()
  const [detail, setDetail] = useState<RoomDetail | null>(null)
  const [message, setMessage] = useState('Loading room...')

  useEffect(() => {
    if (!token || !id) return
    getRoom(token, id)
      .then((next) => {
        setDetail(next)
        setMessage('')
      })
      .catch((error: unknown) =>
        setMessage(
          error instanceof Error ? error.message : 'Failed to load room',
        ),
      )
  }, [id, token])

  if (!detail)
    return (
      <section className="mx-auto w-full max-w-360">
        <Link
          to="/rooms"
          className="text-admin-accent hover:text-admin-accent-bright mb-[1.3rem] inline-flex items-center gap-2 font-mono text-[0.7rem] uppercase no-underline before:content-['<']"
        >
          Back to rooms
        </Link>
        <Notice variant="info" role="alert">
          {message}
        </Notice>
      </section>
    )
  return <RoomDetailPanel detail={detail} />
}

function RoomDetailPanel({ detail }: { detail: RoomDetail }) {
  const { room, live } = detail
  const summary = live.summary
  const connected =
    summary?.players.filter((player) => player.connected).length ?? 0
  const bots = summary?.players.filter((player) => player.is_bot).length ?? 0

  return (
    <section
      className="mx-auto w-full max-w-360"
      aria-labelledby="room-detail-heading"
    >
      <Link
        to="/rooms"
        className="text-admin-accent hover:text-admin-accent-bright mb-[1.3rem] inline-flex items-center gap-2 font-mono text-[0.7rem] uppercase no-underline before:content-['<']"
      >
        Back to rooms
      </Link>

      <header className="flex items-end justify-between gap-8 border-b border-[#f4ead51f] pb-8 max-[760px]:flex-col max-[760px]:items-stretch">
        <div>
          <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase">
            Room record / {room.id}
          </p>
          <div className="flex flex-wrap items-center gap-4">
            <h1
              id="room-detail-heading"
              className="text-admin-ink-strong m-[0.55rem_0_0.65rem] text-[clamp(2.25rem,5vw,4.6rem)] leading-[0.98] font-medium tracking-[-0.055em]"
            >
              {room.name || room.invite_code}
            </h1>
            <RoomStatus value={room.status} />
          </div>
          <p className="text-admin-muted m-0 max-w-175 text-[0.95rem] leading-[1.65]">
            Durable configuration and membership, paired with redacted live
            state from the authoritative game service.
          </p>
        </div>
        <div className="min-w-43.75 rounded-[10px] border border-[#c9922b59] bg-[#c9922b12] p-[0.9rem_1rem] max-[760px]:min-w-0">
          <span className="text-admin-muted-subtle block font-mono text-[0.58rem] uppercase">
            Invite code
          </span>
          <strong className="text-admin-accent-bright m-[0.35rem_0] block font-mono text-[1.2rem] tracking-widest">
            {room.invite_code}
          </strong>
          <small className="text-admin-muted block text-[0.62rem]">
            {formatLabel(room.visibility)} access
          </small>
        </div>
      </header>

      <dl className="[&_.summary-healthy]:text-admin-success [&_dd]:text-admin-ink-strong [&_dt]:text-admin-muted-subtle m-0 grid grid-cols-5 rounded-b-xl border border-t-0 border-[#f4ead51a] bg-[#14241ab3] max-[760px]:grid-cols-2 [&_.summary-warning]:text-[#e0b45e] [&_dd]:m-0 [&_dd]:overflow-hidden [&_dd]:text-[0.82rem] [&_dd]:font-medium [&_dd]:text-ellipsis [&_dd]:whitespace-nowrap [&_dt]:mb-1 [&_dt]:font-mono [&_dt]:text-[0.57rem] [&_dt]:tracking-[0.06em] [&_dt]:uppercase [&>div]:min-w-0 [&>div]:border-r [&>div]:border-[#f4ead517] [&>div]:p-[0.9rem_1rem] max-[760px]:[&>div]:border-b [&>div:last-child]:border-r-0 max-[760px]:[&>div:last-child]:col-span-full max-[760px]:[&>div:last-child]:border-b-0 max-[760px]:[&>div:nth-child(2n)]:border-r-0">
        <SummaryItem
          label="Occupancy"
          value={`${room.player_count} / ${room.max_players}`}
        />
        <SummaryItem label="Mode" value={formatLabel(room.game_mode)} />
        <SummaryItem
          label="Format"
          value={room.practice_mode ? 'Practice' : formatLabel(room.team_mode)}
        />
        <SummaryItem
          label="Live authority"
          value={live.available ? 'Available' : 'Unavailable'}
          tone={live.available ? 'healthy' : 'warning'}
        />
        <SummaryItem
          label="Connections"
          value={live.available ? `${connected} connected` : 'Not reported'}
        />
      </dl>

      <div className="mt-6 grid grid-cols-[minmax(0,1fr)_minmax(310px,380px)] items-start gap-6 max-[1000px]:grid-cols-1">
        <main className="grid min-w-0 gap-6">
          <section
            className="shadow-admin-panel rounded-[14px] border border-[#f4ead51c] bg-[#14241ae0] p-5 [&>div:first-child]:mb-4"
            aria-labelledby="configuration-heading"
          >
            <SectionHeading
              eyebrow="Durable record"
              title="Room configuration"
              id="configuration-heading"
              meta="PostgreSQL"
            />
            <dl className="m-0 grid grid-cols-3 rounded-[9px] border border-[#f4ead517] max-[760px]:grid-cols-2 max-[480px]:grid-cols-1">
              <Configuration
                label="Visibility"
                value={formatLabel(room.visibility)}
                description="Who can discover and enter the room."
              />
              <Configuration
                label="Game mode"
                value={formatLabel(room.game_mode)}
                description={
                  room.practice_mode
                    ? 'Practice rules are enabled.'
                    : 'Standard competitive room.'
                }
              />
              <Configuration
                label="Decks"
                value={String(room.deck_count)}
                description={`${room.max_players} maximum players.`}
              />
              <Configuration
                label="Scoring"
                value={formatLabel(room.scoring_mode)}
                description="Penalty calculation used at game end."
              />
              <Configuration
                label="Teams"
                value={formatLabel(room.team_mode)}
                description={
                  room.team_mode === 'ffa'
                    ? 'Each player competes independently.'
                    : 'Players compete in fixed teams.'
                }
              />
              <Configuration
                label="Turn timer"
                value={
                  room.turn_timer_seconds
                    ? `${room.turn_timer_seconds}s`
                    : 'No limit'
                }
                description="Maximum time allowed for each move."
              />
              <Configuration
                label="Created"
                value={formatDateTime(room.created_at)}
                description={`Owner: ${room.created_by}`}
                wide
              />
            </dl>
          </section>

          <section
            className="shadow-admin-panel rounded-[14px] border border-[#f4ead51c] bg-[#14241ae0] p-5 [&>div:first-child]:mb-4"
            aria-labelledby="players-heading"
          >
            <SectionHeading
              eyebrow="Membership"
              title="Seated players"
              id="players-heading"
              meta={`${detail.players.length} durable members`}
            />
            {detail.players.length ? (
              <div className="grid gap-[0.55rem]">
                {detail.players.map((player, index) => (
                  <DurablePlayer
                    key={player.user_id}
                    player={player}
                    seat={index + 1}
                    live={summary}
                  />
                ))}
              </div>
            ) : (
              <div className="[&_p]:text-admin-muted-subtle rounded-lg border border-dashed border-[#f4ead51f] p-4 text-center [&_p]:m-[0.3rem_0_0] [&_p]:text-[0.7rem] [&_strong]:text-[0.8rem] [&_strong]:text-[#d9d4c8]">
                <strong>No seated players</strong>
                <p>The durable room membership is currently empty.</p>
              </div>
            )}
          </section>
        </main>

        <aside
          className="sticky top-6 max-[1000px]:static"
          aria-labelledby="live-summary-heading"
        >
          <section
            className={`shadow-admin-panel rounded-[14px] border border-[#f4ead51c] bg-[#14241ae0] p-5 ${live.available ? 'border-t-[#2d7a46a6]' : 'border-t-[#c9922b8c]'}`}
          >
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase">
                  Authoritative snapshot
                </p>
                <h2
                  id="live-summary-heading"
                  className="text-admin-ink-strong mt-[0.35rem] mb-0 text-[1.15rem] font-semibold"
                >
                  Live summary
                </h2>
              </div>
              <span
                className={`inline-flex items-center gap-[0.35rem] font-mono text-[0.58rem] uppercase before:size-1.5 before:rounded-full before:bg-current before:content-[''] ${live.available ? 'text-admin-success' : 'text-[#e0b45e]'}`}
              >
                {live.available ? 'Live' : 'Offline'}
              </span>
            </div>
            {!live.available ? (
              <UnavailableState reason={live.reason} />
            ) : summary ? (
              <LiveState summary={summary} bots={bots} />
            ) : (
              <UnavailableState reason="unavailable" />
            )}
            <div className="[&>span]:text-admin-muted mt-4 flex items-start gap-[0.65rem] border-t border-[#f4ead517] pt-[0.9rem] [&_p]:m-[0.2rem_0_0] [&_p]:text-[0.61rem] [&_p]:leading-[1.45] [&_p]:text-[#666b64] [&_strong]:text-[0.67rem] [&_strong]:text-[#b7b3a9] [&>span]:grid [&>span]:size-6 [&>span]:shrink-0 [&>span]:place-items-center [&>span]:rounded-[5px] [&>span]:bg-white/5 [&>span]:font-mono [&>span]:text-[0.6rem]">
              <span aria-hidden="true">R</span>
              <div>
                <strong>Sensitive state redacted</strong>
                <p>
                  Hands, face-down cards, and secret game state are not exposed
                  in this view.
                </p>
              </div>
            </div>
          </section>
        </aside>
      </div>
    </section>
  )
}

function LiveState({
  summary,
  bots,
}: {
  summary: LiveRoomSummary
  bots: number
}) {
  const snapshotTone =
    summary.snapshot_age_seconds <= 10
      ? 'fresh'
      : summary.snapshot_age_seconds <= 60
        ? 'aging'
        : 'stale'
  return (
    <>
      <p className="sr-only">Phase: {summary.phase}</p>
      <div className="[&>small]:text-admin-muted-subtle [&>span]:text-admin-muted-subtle [&>strong]:text-admin-ink mt-[1.2rem] border-l-2 border-[#2d7a46] bg-[#2d7a4614] p-[0.8rem] [&>small]:block [&>small]:text-[0.62rem] [&>span]:block [&>span]:font-mono [&>span]:text-[0.56rem] [&>span]:uppercase [&>strong]:m-[0.28rem_0] [&>strong]:block [&>strong]:text-base">
        <span>Current phase</span>
        <strong>{formatLabel(summary.phase)}</strong>
        <small>{formatLabel(summary.role)} replica response</small>
      </div>
      <dl className="[&_dt]:text-admin-muted-subtle m-[1rem_0_0] grid grid-cols-2 rounded-lg border border-[#f4ead517] [&_dd]:m-[0.25rem_0_0] [&_dd]:overflow-hidden [&_dd]:font-mono [&_dd]:text-[0.7rem] [&_dd]:text-ellipsis [&_dd]:whitespace-nowrap [&_dd]:text-[#d9d4c8] [&_dt]:text-[0.58rem] [&>div]:min-w-0 [&>div]:border-r [&>div]:border-b [&>div]:border-[#f4ead514] [&>div]:p-[0.65rem] [&>div:nth-child(2n)]:border-r-0 [&>div:nth-last-child(-n+2)]:border-b-0">
        <div>
          <dt>Snapshot age</dt>
          <dd
            className={
              snapshotTone === 'fresh'
                ? 'text-admin-success'
                : snapshotTone === 'aging'
                  ? 'text-[#e0b45e]'
                  : 'text-admin-danger'
            }
          >
            {summary.snapshot_age_seconds}s
          </dd>
        </div>
        <div>
          <dt>State version</dt>
          <dd>{summary.state_version}</dd>
        </div>
        <div>
          <dt>Owner replica</dt>
          <dd title={summary.owner_id}>{shortID(summary.owner_id)}</dd>
        </div>
        <div>
          <dt>Fence token</dt>
          <dd>{summary.fence_token}</dd>
        </div>
        <div>
          <dt>Bot seats</dt>
          <dd>{bots}</dd>
        </div>
        <div>
          <dt>Turn deadline</dt>
          <dd>
            {summary.turn_deadline ? formatTime(summary.turn_deadline) : 'None'}
          </dd>
        </div>
      </dl>
      <div className="mt-4 [&_h3]:m-[0_0_0.65rem] [&_h3]:text-[0.72rem] [&_h3]:text-[#d9d4c8]">
        <h3>Live connections</h3>
        {summary.players.map((player, index) => (
          <div
            key={player.user_id}
            className="[&_em]:text-admin-muted-subtle grid grid-cols-[8px_minmax(0,1fr)_auto] items-center gap-[0.55rem] border-t border-[#f4ead512] py-[0.55rem] [&_em]:font-mono [&_em]:text-[0.55rem] [&_em]:uppercase [&_em]:not-italic [&_small]:mt-[0.15rem] [&_small]:block [&_small]:text-[0.57rem] [&_small]:text-[#666b64] [&_strong]:block [&_strong]:text-[0.7rem] [&_strong]:text-[#d9d4c8]"
          >
            <span className="sr-only">
              {player.display_name} ·{' '}
              {player.connected ? 'connected' : 'disconnected'}
              {player.is_bot ? ' · bot' : ''}
            </span>
            <span
              className={`size-1.5 rounded-full ${player.connected ? 'bg-[#56b875] shadow-[0_0_0_3px_rgb(45_122_70/14%)]' : 'bg-[#665d57]'}`}
            />
            <div>
              <strong>{player.display_name}</strong>
              <small>
                Seat {player.seat ?? index + 1}
                {player.is_bot ? ' / Bot' : ''}
              </small>
            </div>
            <em>{player.connected ? 'connected' : 'disconnected'}</em>
          </div>
        ))}
      </div>
    </>
  )
}

function UnavailableState({ reason }: { reason?: string }) {
  return (
    <div className="[&_p]:text-admin-muted-subtle mt-[1.2rem] grid rounded-lg border border-dashed border-[#c9922b59] p-4 [&_p]:m-[0.35rem_0_0] [&_p]:text-[0.68rem] [&_p]:leading-normal [&_strong]:mt-3 [&_strong]:text-[0.76rem] [&_strong]:text-[#ffaaa4] [&>span]:grid [&>span]:size-7 [&>span]:place-items-center [&>span]:rounded-full [&>span]:border [&>span]:border-[#c9922b73] [&>span]:font-mono [&>span]:text-[#e0b45e]">
      <span aria-hidden="true">!</span>
      <strong>
        Live room state unavailable
        {reason ? `: ${reason.replaceAll('_', ' ')}` : ''}.
      </strong>
      <p>
        The durable record is intact. Live ownership may have ended, moved, or
        be temporarily unreachable.
      </p>
    </div>
  )
}

function DurablePlayer({
  player,
  seat,
  live,
}: {
  player: RoomPlayer
  seat: number
  live?: LiveRoomSummary
}) {
  const livePlayer = live?.players.find(
    (candidate) => candidate.user_id === player.user_id,
  )
  return (
    <article className="grid grid-cols-[32px_38px_minmax(0,1fr)_auto] items-center gap-[0.7rem] rounded-lg border border-[#f4ead517] p-[0.7rem] max-[760px]:grid-cols-[28px_34px_minmax(0,1fr)]">
      <span className="font-mono text-[0.62rem] text-[#5f665e]">
        {String(seat).padStart(2, '0')}
      </span>
      <span className="text-admin-ink grid size-8.5 place-items-center rounded-full bg-[#235c36] text-[0.68rem] font-semibold">
        {initials(player.display_name)}
      </span>
      <div>
        <strong className="text-admin-ink text-[0.78rem]">
          {player.display_name}
        </strong>
        <p className="text-admin-muted-subtle m-[0.2rem_0_0] max-w-70 overflow-hidden font-mono text-[0.56rem] text-ellipsis whitespace-nowrap">
          {player.user_id}
        </p>
      </div>
      <div className="grid justify-items-end gap-[0.18rem] max-[760px]:col-start-3 max-[760px]:justify-items-start">
        <span
          className={`text-[0.64rem] ${livePlayer?.connected ? 'text-admin-success' : 'text-admin-muted'}`}
        >
          {livePlayer
            ? livePlayer.connected
              ? 'Connected'
              : 'Disconnected'
            : 'No live state'}
        </span>
        <small className="text-[0.57rem] text-[#60645e]">
          Joined {formatDateTime(player.joined_at)}
        </small>
      </div>
    </article>
  )
}

function Configuration({
  label,
  value,
  description,
  wide = false,
}: {
  label: string
  value: string
  description: string
  wide?: boolean
}) {
  return (
    <div
      className={`[&_dd]:text-admin-ink [&_dt]:text-admin-muted-subtle [&_p]:text-admin-muted-subtle min-w-0 border-r border-b border-[#f4ead514] p-[0.9rem] nth-[3n]:border-r-0 max-[760px]:nth-[2n]:border-r-0 max-[760px]:nth-[3n]:border-r max-[480px]:border-r-0 [&_dd]:m-[0.35rem_0_0] [&_dd]:text-[0.84rem] [&_dd]:font-semibold [&_dt]:font-mono [&_dt]:text-[0.57rem] [&_dt]:uppercase [&_p]:m-[0.35rem_0_0] [&_p]:text-[0.65rem] [&_p]:leading-[1.4] ${wide ? 'col-span-3 border-r-0 border-b-0 max-[760px]:col-span-2 max-[480px]:col-span-1' : ''}`}
    >
      <dt>{label}</dt>
      <dd>{value}</dd>
      <p>{description}</p>
    </div>
  )
}

function initials(name: string) {
  return (
    name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase())
      .join('') || '?'
  )
}
