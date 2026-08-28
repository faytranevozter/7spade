import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { getRoom, type LiveRoomSummary, type RoomDetail, type RoomPlayer } from '../api/rooms'
import { Notice } from '../components/Feedback'
import { RoomStatus, SectionHeading, SummaryItem } from '../components/InvestigationUI'
import { formatDateTime, formatLabel, formatTime, shortID } from '../components/formatters'
import { useAuth } from '../hooks/useAuth'

export function RoomDetailPage() {
  const { id = '' } = useParams()
  const { token } = useAuth()
  const [detail, setDetail] = useState<RoomDetail | null>(null)
  const [message, setMessage] = useState('Loading room...')

  useEffect(() => {
    if (!token || !id) return
    getRoom(token, id).then((next) => {
      setDetail(next)
      setMessage('')
    }).catch((error: unknown) => setMessage(error instanceof Error ? error.message : 'Failed to load room'))
  }, [id, token])

  if (!detail) return <section className="room-detail-page"><Link to="/rooms" className="back-link">Back to rooms</Link><Notice variant="info" role="alert">{message}</Notice></section>
  return <RoomDetailPanel detail={detail} />
}

function RoomDetailPanel({ detail }: { detail: RoomDetail }) {
  const { room, live } = detail
  const summary = live.summary
  const connected = summary?.players.filter((player) => player.connected).length ?? 0
  const bots = summary?.players.filter((player) => player.is_bot).length ?? 0

  return <section className="room-detail-page" aria-labelledby="room-detail-heading">
    <Link to="/rooms" className="back-link">Back to rooms</Link>

    <header className="room-detail-hero">
      <div>
        <p className="eyebrow">Room record / {room.id}</p>
        <div className="room-detail-title"><h1 id="room-detail-heading">{room.name || room.invite_code}</h1><RoomStatus value={room.status} /></div>
        <p>Durable configuration and membership, paired with redacted live state from the authoritative game service.</p>
      </div>
      <div className="invite-code-card"><span>Invite code</span><strong>{room.invite_code}</strong><small>{formatLabel(room.visibility)} access</small></div>
    </header>

    <dl className="room-summary-strip">
      <SummaryItem label="Occupancy" value={`${room.player_count} / ${room.max_players}`} />
      <SummaryItem label="Mode" value={formatLabel(room.game_mode)} />
      <SummaryItem label="Format" value={room.practice_mode ? 'Practice' : formatLabel(room.team_mode)} />
      <SummaryItem label="Live authority" value={live.available ? 'Available' : 'Unavailable'} tone={live.available ? 'healthy' : 'warning'} />
      <SummaryItem label="Connections" value={live.available ? `${connected} connected` : 'Not reported'} />
    </dl>

    <div className="room-detail-layout">
      <main className="room-record-column">
        <section className="room-record-section" aria-labelledby="configuration-heading">
          <SectionHeading eyebrow="Durable record" title="Room configuration" id="configuration-heading" meta="PostgreSQL" />
          <dl className="configuration-grid">
            <Configuration label="Visibility" value={formatLabel(room.visibility)} description="Who can discover and enter the room." />
            <Configuration label="Game mode" value={formatLabel(room.game_mode)} description={room.practice_mode ? 'Practice rules are enabled.' : 'Standard competitive room.'} />
            <Configuration label="Decks" value={String(room.deck_count)} description={`${room.max_players} maximum players.`} />
            <Configuration label="Scoring" value={formatLabel(room.scoring_mode)} description="Penalty calculation used at game end." />
            <Configuration label="Teams" value={formatLabel(room.team_mode)} description={room.team_mode === 'ffa' ? 'Each player competes independently.' : 'Players compete in fixed teams.'} />
            <Configuration label="Turn timer" value={room.turn_timer_seconds ? `${room.turn_timer_seconds}s` : 'No limit'} description="Maximum time allowed for each move." />
            <Configuration label="Created" value={formatDateTime(room.created_at)} description={`Owner: ${room.created_by}`} wide />
          </dl>
        </section>

        <section className="room-record-section" aria-labelledby="players-heading">
          <SectionHeading eyebrow="Membership" title="Seated players" id="players-heading" meta={`${detail.players.length} durable members`} />
          {detail.players.length ? <div className="durable-player-list">{detail.players.map((player, index) => <DurablePlayer key={player.user_id} player={player} seat={index + 1} live={summary} />)}</div> : <div className="room-inline-empty"><strong>No seated players</strong><p>The durable room membership is currently empty.</p></div>}
        </section>
      </main>

      <aside className="live-room-rail" aria-labelledby="live-summary-heading">
        <section className={`live-summary-panel ${live.available ? 'live-available' : 'live-unavailable'}`}>
          <div className="live-panel-heading"><div><p className="eyebrow">Authoritative snapshot</p><h2 id="live-summary-heading">Live summary</h2></div><span className="live-indicator">{live.available ? 'Live' : 'Offline'}</span></div>
          {!live.available ? <UnavailableState reason={live.reason} /> : summary ? <LiveState summary={summary} bots={bots} /> : <UnavailableState reason="unavailable" />}
          <div className="redaction-notice"><span aria-hidden="true">R</span><div><strong>Sensitive state redacted</strong><p>Hands, face-down cards, and secret game state are not exposed in this view.</p></div></div>
        </section>
      </aside>
    </div>
  </section>
}

function LiveState({ summary, bots }: { summary: LiveRoomSummary; bots: number }) {
  const snapshotTone = summary.snapshot_age_seconds <= 10 ? 'fresh' : summary.snapshot_age_seconds <= 60 ? 'aging' : 'stale'
  return <>
    <p className="sr-only">Phase: {summary.phase}</p>
    <div className="live-phase"><span>Current phase</span><strong>{formatLabel(summary.phase)}</strong><small>{formatLabel(summary.role)} replica response</small></div>
    <dl className="live-metrics">
      <div><dt>Snapshot age</dt><dd className={`snapshot-${snapshotTone}`}>{summary.snapshot_age_seconds}s</dd></div>
      <div><dt>State version</dt><dd>{summary.state_version}</dd></div>
      <div><dt>Owner replica</dt><dd title={summary.owner_id}>{shortID(summary.owner_id)}</dd></div>
      <div><dt>Fence token</dt><dd>{summary.fence_token}</dd></div>
      <div><dt>Bot seats</dt><dd>{bots}</dd></div>
      <div><dt>Turn deadline</dt><dd>{summary.turn_deadline ? formatTime(summary.turn_deadline) : 'None'}</dd></div>
    </dl>
    <div className="connection-list">
      <h3>Live connections</h3>
      {summary.players.map((player, index) => <div key={player.user_id} className="connection-row"><span className="sr-only">{player.display_name} · {player.connected ? 'connected' : 'disconnected'}{player.is_bot ? ' · bot' : ''}</span><span className={`connection-dot ${player.connected ? 'connected' : ''}`} /><div><strong>{player.display_name}</strong><small>Seat {player.seat ?? index + 1}{player.is_bot ? ' / Bot' : ''}</small></div><em>{player.connected ? 'connected' : 'disconnected'}</em></div>)}
    </div>
  </>
}

function UnavailableState({ reason }: { reason?: string }) {
  return <div className="live-unavailable-state"><span aria-hidden="true">!</span><strong>Live room state unavailable{reason ? `: ${reason.replaceAll('_', ' ')}` : ''}.</strong><p>The durable record is intact. Live ownership may have ended, moved, or be temporarily unreachable.</p></div>
}

function DurablePlayer({ player, seat, live }: { player: RoomPlayer; seat: number; live?: LiveRoomSummary }) {
  const livePlayer = live?.players.find((candidate) => candidate.user_id === player.user_id)
  return <article className="durable-player-card"><span className="seat-number">{String(seat).padStart(2, '0')}</span><span className="player-avatar-mark">{initials(player.display_name)}</span><div><strong>{player.display_name}</strong><p>{player.user_id}</p></div><div className="player-membership-meta"><span className={livePlayer?.connected ? 'member-connected' : ''}>{livePlayer ? (livePlayer.connected ? 'Connected' : 'Disconnected') : 'No live state'}</span><small>Joined {formatDateTime(player.joined_at)}</small></div></article>
}

function Configuration({ label, value, description, wide = false }: { label: string; value: string; description: string; wide?: boolean }) {
  return <div className={wide ? 'configuration-item configuration-wide' : 'configuration-item'}><dt>{label}</dt><dd>{value}</dd><p>{description}</p></div>
}

function initials(name: string) {
  return name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]?.toUpperCase()).join('') || '?'
}

