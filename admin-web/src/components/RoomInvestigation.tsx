import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { searchRooms, type Room, type RoomFilters } from '../api/rooms'

export function RoomInvestigation({ token }: { token: string }) {
  const [filters, setFilters] = useState<RoomFilters>({})
  const [rooms, setRooms] = useState<Room[]>([])
  const [message, setMessage] = useState('')
  const [offset, setOffset] = useState(0)
  const pageSize = 50

  useEffect(() => {
    let cancelled = false
    searchRooms(token, filters, pageSize, offset).then((page) => {
      if (!cancelled) { setRooms(page.rooms); setMessage('') }
    }).catch((error: unknown) => {
      if (!cancelled) setMessage(error instanceof Error ? error.message : 'Failed to search rooms')
    })
    return () => { cancelled = true }
  }, [filters, offset, token])

  function update(key: keyof RoomFilters, value: string) { setFilters((current) => ({ ...current, [key]: value })); setOffset(0) }
  const inputClass = 'bg-[#10161d] border border-[#28323d] text-white px-3 py-2 focus:outline-none focus:border-[#4dd0b5]'
  return <section className="mt-8 grid gap-6" aria-labelledby="rooms-heading">
    <header className="border-b border-[#28323d] pb-5">
      <p className="m-0 text-[#738397] font-mono font-bold text-[11px] tracking-[0.13em]">ROOM INVESTIGATION</p>
      <h2 id="rooms-heading" className="mt-2 text-xl font-bold text-white">Rooms</h2>
      <p className="mt-2 text-sm text-[#8493a5]">Search durable room records and inspect redacted live state when available.</p>
    </header>
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      <label className="grid gap-1 text-sm text-[#aeb8c4]">Room ID<input value={filters.id ?? ''} onChange={(event) => update('id', event.target.value)} className={inputClass} /></label>
      <label className="grid gap-1 text-sm text-[#aeb8c4]">Invite code<input value={filters.invite_code ?? ''} onChange={(event) => update('invite_code', event.target.value)} className={inputClass} /></label>
      <label className="grid gap-1 text-sm text-[#aeb8c4]">Status<select value={filters.status ?? ''} onChange={(event) => update('status', event.target.value)} className={inputClass}><option value="">Any</option><option value="waiting">Waiting</option><option value="in_progress">In progress</option><option value="finished">Finished</option></select></label>
      <label className="grid gap-1 text-sm text-[#aeb8c4]">Visibility<select value={filters.visibility ?? ''} onChange={(event) => update('visibility', event.target.value)} className={inputClass}><option value="">Any</option><option value="public">Public</option><option value="private">Private</option></select></label>
      <label className="grid gap-1 text-sm text-[#aeb8c4]">Mode<input value={filters.mode ?? ''} onChange={(event) => update('mode', event.target.value)} placeholder="classic" className={inputClass} /></label>
      <label className="grid gap-1 text-sm text-[#aeb8c4]">Created after<input type="datetime-local" value={filters.created_from ?? ''} onChange={(event) => update('created_from', event.target.value ? new Date(event.target.value).toISOString() : '')} className={inputClass} /></label>
      <label className="grid gap-1 text-sm text-[#aeb8c4]">Created before<input type="datetime-local" value={filters.created_to ?? ''} onChange={(event) => update('created_to', event.target.value ? new Date(event.target.value).toISOString() : '')} className={inputClass} /></label>
    </div>
    {message ? <p role="alert" className="text-[#ffaaa4]">{message}</p> : null}
    <div className="flex items-center gap-3"><button type="button" onClick={() => setOffset((value) => Math.max(0, value - pageSize))} disabled={offset === 0} className="border border-[#394552] px-3 py-2 text-sm text-[#aeb8c4] disabled:opacity-50">Previous</button><span className="text-sm text-[#8493a5]">Page {Math.floor(offset / pageSize) + 1}</span><button type="button" onClick={() => setOffset((value) => value + pageSize)} disabled={rooms.length < pageSize} className="border border-[#4dd0b5] px-3 py-2 text-sm text-[#4dd0b5] disabled:opacity-50">Next</button></div>
    <div className="grid gap-3">{rooms.map((room) => <Link key={room.id} to={`/rooms/${room.id}`} className="border border-[#28323d] bg-[#10161d] p-4 no-underline text-[#aeb8c4] hover:border-[#4dd0b5]"><strong className="text-white">{room.name || room.invite_code}</strong><span className="ml-3 font-mono text-sm">{room.invite_code}</span><p className="mt-2 text-sm">{room.status.replace('_', ' ')} · {room.visibility} · {room.game_mode} · {room.practice_mode ? 'practice' : 'standard'} · {room.player_count}/{room.max_players} seated</p></Link>)}{rooms.length === 0 ? <p className="text-[#8493a5]">No rooms found.</p> : null}</div>
  </section>
}
