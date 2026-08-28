import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { getRoom, type RoomDetail } from '../api/rooms'
import { useAuth } from '../hooks/useAuth'

export function RoomDetailPage() {
  const { id = '' } = useParams()
  const { token } = useAuth()
  const [detail, setDetail] = useState<RoomDetail | null>(null)
  const [message, setMessage] = useState('Loading room...')
  useEffect(() => {
    if (!token || !id) return
    getRoom(token, id).then((next) => { setDetail(next); setMessage('') }).catch((error: unknown) => setMessage(error instanceof Error ? error.message : 'Failed to load room'))
  }, [id, token])
  return <section aria-labelledby="room-detail-heading"><Link to="/rooms" className="text-sm text-[#4dd0b5] underline">Back to rooms</Link>{message ? <p role="alert" className="mt-4 text-[#ffaaa4]">{message}</p> : null}{detail ? <RoomDetailPanel detail={detail} /> : null}</section>
}

function RoomDetailPanel({ detail }: { detail: RoomDetail }) {
  const { room, live } = detail
  const summary = live.summary
  return <section className="mt-5 border border-[#28323d] bg-[#10161d] p-5" aria-labelledby="room-detail-heading"><p className="m-0 text-[#738397] font-mono font-bold text-[11px] tracking-[0.13em]">ROOM DETAIL</p><h2 id="room-detail-heading" className="mt-2 text-xl font-bold text-white">{room.name || room.invite_code}</h2><p className="text-[#8493a5] break-all">{room.id}</p><dl className="grid grid-cols-1 gap-3 text-sm text-[#aeb8c4] sm:grid-cols-2"><Detail label="Invite code" value={room.invite_code} /><Detail label="Status" value={room.status} /><Detail label="Visibility" value={room.visibility} /><Detail label="Mode" value={room.game_mode} /><Detail label="Practice" value={room.practice_mode ? 'Yes' : 'No'} /><Detail label="Seats" value={`${room.player_count}/${room.max_players}`} /><Detail label="Created" value={new Date(room.created_at).toLocaleString()} /><Detail label="Owner" value={room.created_by} /></dl><section className="mt-6 border-t border-[#28323d] pt-4"><h3 className="text-sm font-bold text-white">Seated players</h3>{detail.players.length ? <ul className="mt-2 grid gap-1 pl-5 text-sm text-[#aeb8c4]">{detail.players.map((player) => <li key={player.user_id}>{player.display_name} · {player.user_id}</li>)}</ul> : <p className="text-sm text-[#8493a5]">No seated players.</p>}</section><section className="mt-6 border-t border-[#28323d] pt-4"><h3 className="text-sm font-bold text-white">Live summary</h3>{!live.available ? <p className="text-sm text-[#ffaaa4]">Live room state unavailable{live.reason ? `: ${live.reason.replace('_', ' ')}` : ''}.</p> : <div className="grid gap-3 text-sm text-[#aeb8c4]"><p>Phase: {summary?.phase} · Owner: {summary?.owner_id} · Fence: {summary?.fence_token}</p><p>State version: {summary?.state_version} · Snapshot age: {summary?.snapshot_age_seconds}s{summary?.turn_deadline ? ` · Turn deadline: ${new Date(summary.turn_deadline).toLocaleString()}` : ''}</p><ul className="grid gap-1 pl-5">{summary?.players.map((player) => <li key={player.user_id}>{player.display_name} · {player.connected ? 'connected' : 'disconnected'}{player.is_bot ? ' · bot' : ''}</li>)}</ul></div>}</section></section>
}
function Detail({ label, value }: { label: string; value: string }) { return <div><dt className="text-[#738397]">{label}</dt><dd className="m-0 break-all">{value}</dd></div> }
