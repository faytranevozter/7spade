import { Button } from './Button'
import type { Room } from '../types'

export function RoomCard({ room }: { room: Room }) {
  return (
    <article className={`flex items-center gap-3 rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-3 transition hover:border-spade-cream/18 ${room.open ? '' : 'opacity-55'}`}>
      <span className={`size-2.5 rounded-full ${room.open ? 'bg-spade-green-light' : 'bg-spade-gray-3'}`} />
      <div className="min-w-0 flex-1">
        <h3 className="truncate text-sm font-medium text-spade-cream">{room.name}</h3>
        <p className="font-mono text-[11px] text-spade-gray-3">
          {room.players} players · {room.timer} · {room.code}
        </p>
        <p className="text-xs text-spade-gray-2">{room.status}</p>
      </div>
      <Button variant={room.open ? 'primary' : 'secondary'} disabled={!room.open} className="min-h-8 px-3 py-1.5 text-xs">
        {room.open ? 'Join' : 'Full'}
      </Button>
    </article>
  )
}
