import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import {
  getRoom,
  type LiveRoomSummary,
  type RoomDetail,
  type RoomPlayer,
} from '../api/rooms'
import { AdminPage, AdminPageHeader, AdminPanel } from '../components/AdminPage'
import { Notice } from '../components/Feedback'
import { RoomStatus, SectionHeading } from '../components/InvestigationUI'
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
  const [message, setMessage] = useState('')

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
      <AdminPage>
        <Link
          to="/rooms"
          className="text-admin-accent hover:text-admin-accent-bright text-admin-field mb-admin-control-link inline-flex items-center gap-2 font-mono uppercase no-underline before:content-['<']"
        >
          Back to rooms
        </Link>
        <Notice variant={message ? 'error' : 'info'}>
          {message || 'Loading room...'}
        </Notice>
      </AdminPage>
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
    <AdminPage labelledBy="room-detail-heading">
      <AdminPageHeader
        variant="detail"
        titleId="room-detail-heading"
        eyebrow={`Room record / ${room.id}`}
        title={
          <span className="flex flex-wrap items-center gap-4">
            <span>{room.name || room.invite_code}</span>
            <RoomStatus value={room.status} />
          </span>
        }
        description={
          <p className="m-0 max-w-175">
            Durable configuration and membership, paired with redacted live
            state from the authoritative game service.
          </p>
        }
        backLink={
          <Link
            to="/rooms"
            className="text-admin-accent hover:text-admin-accent-bright text-admin-field inline-flex items-center gap-2 font-mono uppercase no-underline before:content-['<']"
          >
            Back to rooms
          </Link>
        }
        actions={
          <div className="rounded-admin-preview bg-admin-accent-faint border-admin-accent-border-subtle min-w-43.75 border p-[0.9rem_1rem] max-[760px]:min-w-0">
            <span className="text-admin-muted-subtle text-admin-caption block font-mono uppercase">
              Invite code
            </span>
            <strong className="text-admin-accent-bright text-admin-heading m-[0.35rem_0] block font-mono tracking-widest">
              {room.invite_code}
            </strong>
            <small className="text-admin-muted text-admin-caption block">
              {formatLabel(room.visibility)} access
            </small>
          </div>
        }
      />

      <dl className="border-admin-border-section bg-admin-surface-translucent m-0 grid grid-cols-5 rounded-b-xl border border-t-0 max-[760px]:grid-cols-2">
        {[
          ['Occupancy', `${room.player_count} / ${room.max_players}`],
          ['Mode', formatLabel(room.game_mode)],
          [
            'Format',
            room.practice_mode ? 'Practice' : formatLabel(room.team_mode),
          ],
          ['Live authority', live.available ? 'Available' : 'Unavailable'],
          [
            'Connections',
            live.available ? `${connected} connected` : 'Not reported',
          ],
        ].map(([label, value], index) => (
          <div
            key={label}
            className={`border-admin-border-faint min-w-0 border-r p-[0.9rem_1rem] max-[760px]:border-b ${index === 4 ? 'border-r-0 max-[760px]:col-span-full max-[760px]:border-b-0' : ''} ${index % 2 === 1 ? 'max-[760px]:border-r-0' : ''}`}
          >
            <dt className="text-admin-muted-subtle text-admin-xs mb-1 font-mono tracking-[0.06em] uppercase">
              {label}
            </dt>
            <dd
              className={`text-admin-ink-strong text-admin-field m-0 overflow-hidden font-medium text-ellipsis whitespace-nowrap ${label === 'Live authority' ? (live.available ? 'text-admin-success' : 'text-admin-warning') : ''}`}
            >
              {value}
            </dd>
          </div>
        ))}
      </dl>

      <div className="mt-6 grid grid-cols-[minmax(0,1fr)_minmax(310px,380px)] items-start gap-6 max-[1000px]:grid-cols-1">
        <main className="grid min-w-0 gap-6">
          <section aria-labelledby="configuration-heading">
            <AdminPanel>
              <div className="mb-4">
                <SectionHeading
                  eyebrow="Durable record"
                  title="Room configuration"
                  id="configuration-heading"
                  meta="PostgreSQL"
                />
              </div>
              <dl className="rounded-admin-rule border-admin-border-faint m-0 grid grid-cols-3 border max-[760px]:grid-cols-2 max-[480px]:grid-cols-1">
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
            </AdminPanel>
          </section>

          <section aria-labelledby="players-heading">
            <AdminPanel>
              <div className="mb-4">
                <SectionHeading
                  eyebrow="Membership"
                  title="Seated players"
                  id="players-heading"
                  meta={`${detail.players.length} durable members`}
                />
              </div>
              {detail.players.length ? (
                <div className="gap-admin-7 grid">
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
                <div className="border-admin-border rounded-lg border border-dashed p-4 text-center">
                  <strong className="text-admin-ink-soft text-admin-action">
                    No seated players
                  </strong>
                  <p className="text-admin-muted-subtle text-admin-field m-[0.3rem_0_0]">
                    The durable room membership is currently empty.
                  </p>
                </div>
              )}
            </AdminPanel>
          </section>
        </main>

        <aside
          className="sticky top-6 max-[1000px]:static"
          aria-labelledby="live-summary-heading"
        >
          <AdminPanel
            className={
              live.available
                ? 'border-t-admin-success-border'
                : 'border-t-admin-accent-border'
            }
          >
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="text-admin-accent text-admin-note m-0 font-mono font-medium tracking-[0.13em] uppercase">
                  Authoritative snapshot
                </p>
                <h2
                  id="live-summary-heading"
                  className="text-admin-ink-strong mt-admin-4 text-admin-section mb-0 font-semibold"
                >
                  Live summary
                </h2>
              </div>
              <span
                className={`text-admin-caption gap-admin-4 inline-flex items-center font-mono uppercase before:size-1.5 before:rounded-full before:bg-current before:content-[''] ${live.available ? 'text-admin-success' : 'text-admin-warning'}`}
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
            <div className="gap-admin-9 border-admin-border-faint pt-admin-15 mt-4 flex items-start border-t">
              <span
                aria-hidden="true"
                className="text-admin-muted text-admin-label rounded-admin-control grid size-6 shrink-0 place-items-center bg-white/5 font-mono"
              >
                R
              </span>
              <div>
                <strong className="text-admin-small text-admin-ink-soft">
                  Sensitive state redacted
                </strong>
                <p className="text-admin-muted-subtle text-admin-meta-small m-[0.2rem_0_0] leading-[1.45]">
                  Hands, face-down cards, and secret game state are not exposed
                  in this view.
                </p>
              </div>
            </div>
          </AdminPanel>
        </aside>
      </div>
    </AdminPage>
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
      <div className="mt-admin-17 p-admin-13 border-admin-success bg-admin-success-bg border-l-2">
        <span className="text-admin-muted-subtle text-admin-caption block font-mono uppercase">
          Current phase
        </span>
        <strong className="text-admin-ink m-[0.28rem_0] block text-base">
          {formatLabel(summary.phase)}
        </strong>
        <small className="text-admin-muted-subtle text-admin-caption block">
          {formatLabel(summary.role)} replica response
        </small>
      </div>
      <dl className="border-admin-border-faint m-[1rem_0_0] grid grid-cols-2 rounded-lg border">
        <div className="border-admin-border-divider p-admin-9 min-w-0 border-r border-b">
          <dt className="text-admin-muted-subtle text-admin-caption">
            Snapshot age
          </dt>
          <dd
            className={`text-admin-field m-[0.25rem_0_0] overflow-hidden font-mono text-ellipsis whitespace-nowrap ${snapshotTone === 'fresh' ? 'text-admin-success' : snapshotTone === 'aging' ? 'text-admin-warning' : 'text-admin-danger'}`}
          >
            {summary.snapshot_age_seconds}s
          </dd>
        </div>
        <div className="border-admin-border-divider p-admin-9 min-w-0 border-b">
          <dt className="text-admin-muted-subtle text-admin-caption">
            State version
          </dt>
          <dd className="text-admin-ink-soft text-admin-field m-[0.25rem_0_0] overflow-hidden font-mono text-ellipsis whitespace-nowrap">
            {summary.state_version}
          </dd>
        </div>
        <div className="border-admin-border-divider p-admin-9 min-w-0 border-r border-b">
          <dt className="text-admin-muted-subtle text-admin-caption">
            Owner replica
          </dt>
          <dd
            title={summary.owner_id}
            className="text-admin-ink-soft text-admin-field m-[0.25rem_0_0] overflow-hidden font-mono text-ellipsis whitespace-nowrap"
          >
            {shortID(summary.owner_id)}
          </dd>
        </div>
        <div className="border-admin-border-divider p-admin-9 min-w-0 border-b">
          <dt className="text-admin-muted-subtle text-admin-caption">
            Fence token
          </dt>
          <dd className="text-admin-ink-soft text-admin-field m-[0.25rem_0_0] overflow-hidden font-mono text-ellipsis whitespace-nowrap">
            {summary.fence_token}
          </dd>
        </div>
        <div className="border-admin-border-divider p-admin-9 min-w-0 border-r">
          <dt className="text-admin-muted-subtle text-admin-caption">
            Bot seats
          </dt>
          <dd className="text-admin-ink-soft text-admin-field m-[0.25rem_0_0] overflow-hidden font-mono text-ellipsis whitespace-nowrap">
            {bots}
          </dd>
        </div>
        <div className="p-admin-9 min-w-0">
          <dt className="text-admin-muted-subtle text-admin-caption">
            Turn deadline
          </dt>
          <dd className="text-admin-ink-soft text-admin-field m-[0.25rem_0_0] overflow-hidden font-mono text-ellipsis whitespace-nowrap">
            {summary.turn_deadline ? formatTime(summary.turn_deadline) : 'None'}
          </dd>
        </div>
      </dl>
      <div className="mt-4">
        <h3 className="text-admin-ink-soft text-admin-field m-[0_0_0.65rem]">
          Live connections
        </h3>
        {summary.players.map((player, index) => (
          <div
            key={player.user_id}
            className="gap-admin-7 py-admin-7 border-admin-border-divider grid grid-cols-[8px_minmax(0,1fr)_auto] items-center border-t"
          >
            <span className="sr-only">
              {player.display_name} ·{' '}
              {player.connected ? 'connected' : 'disconnected'}
              {player.is_bot ? ' · bot' : ''}
            </span>
            <span
              className={`size-1.5 rounded-full ${player.connected ? 'bg-admin-success shadow-admin-focus' : 'bg-admin-muted-subtle'}`}
            />
            <div>
              <strong className="text-admin-ink-soft text-admin-field block">
                {player.display_name}
              </strong>
              <small className="mt-admin-1 text-admin-xs text-admin-muted-subtle block">
                Seat {player.seat ?? index + 1}
                {player.is_bot ? ' / Bot' : ''}
              </small>
            </div>
            <em className="text-admin-muted-subtle text-admin-xs font-mono uppercase not-italic">
              {player.connected ? 'connected' : 'disconnected'}
            </em>
          </div>
        ))}
      </div>
    </>
  )
}

function UnavailableState({ reason }: { reason?: string }) {
  return (
    <div className="mt-admin-17 border-admin-accent-border-subtle grid rounded-lg border border-dashed p-4">
      <span
        aria-hidden="true"
        className="border-admin-accent-border text-admin-warning grid size-7 place-items-center rounded-full border font-mono"
      >
        !
      </span>
      <strong className="text-admin-body text-admin-danger mt-3">
        Live room state unavailable
        {reason ? `: ${reason.replaceAll('_', ' ')}` : ''}.
      </strong>
      <p className="text-admin-muted-subtle text-admin-note m-[0.35rem_0_0] leading-normal">
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
    <article className="gap-admin-10 border-admin-border-faint p-admin-10 grid grid-cols-[32px_38px_minmax(0,1fr)_auto] items-center rounded-lg border max-[760px]:grid-cols-[28px_34px_minmax(0,1fr)]">
      <span className="text-admin-caption text-admin-muted-subtle font-mono">
        {String(seat).padStart(2, '0')}
      </span>
      <span className="text-admin-ink bg-admin-surface-raised text-admin-note grid size-8.5 place-items-center rounded-full font-semibold">
        {initials(player.display_name)}
      </span>
      <div>
        <strong className="text-admin-ink text-admin-action">
          {player.display_name}
        </strong>
        <p className="text-admin-muted-subtle text-admin-caption m-[0.2rem_0_0] max-w-70 overflow-hidden font-mono text-ellipsis whitespace-nowrap">
          {player.user_id}
        </p>
      </div>
      <div className="gap-admin-badge-y grid justify-items-end max-[760px]:col-start-3 max-[760px]:justify-items-start">
        <span
          className={`text-admin-meta ${livePlayer?.connected ? 'text-admin-success' : 'text-admin-muted'}`}
        >
          {livePlayer
            ? livePlayer.connected
              ? 'Connected'
              : 'Disconnected'
            : 'No live state'}
        </span>
        <small className="text-admin-xs text-admin-muted-subtle">
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
      className={`border-admin-border-divider p-admin-15 min-w-0 border-r border-b nth-[3n]:border-r-0 max-[760px]:nth-[2n]:border-r-0 max-[760px]:nth-[3n]:border-r max-[480px]:border-r-0 ${wide ? 'col-span-3 border-r-0 border-b-0 max-[760px]:col-span-2 max-[480px]:col-span-1' : ''}`}
    >
      <dt className="text-admin-muted-subtle text-admin-xs font-mono uppercase">
        {label}
      </dt>
      <dd className="text-admin-ink text-admin-action m-[0.35rem_0_0] font-semibold">
        {value}
      </dd>
      <p className="text-admin-muted-subtle text-admin-meta m-[0.35rem_0_0] leading-[1.4]">
        {description}
      </p>
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
