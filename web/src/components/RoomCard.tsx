import { Badge } from './Badge'
import { Button } from './Button'
import type { Room } from '../types'

export function RoomCard({ room, onJoin }: { room: Room; onJoin?: () => void }) {
  const seats = Array.from({ length: room.maxSeats }, (_, i) => i < room.filledSeats)

  return (
    <article
      className={`flex flex-col gap-3 rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4 transition hover:border-spade-cream/20 ${room.open ? '' : 'opacity-60'}`}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <h3 className="truncate text-sm font-medium text-spade-cream">{room.name}</h3>
          <p className="mt-0.5 text-xs text-spade-gray-2">{room.status}</p>
        </div>
        <Badge tone={room.open ? 'playing' : 'passed'}>{room.open ? 'Open' : 'Full'}</Badge>
      </div>

      <div className="flex items-center gap-1.5" aria-label={`${room.filledSeats} of ${room.maxSeats} seats filled`}>
        {seats.map((filled, i) => (
          <span
            key={i}
            className={`h-2 flex-1 rounded-full ${filled ? 'bg-spade-green-light' : 'bg-spade-cream/12'}`}
          />
        ))}
      </div>

      <div className="flex items-center justify-between gap-2 font-mono text-[11px] text-spade-gray-3">
        <span className="rounded-spade-sm border border-spade-gold/30 bg-spade-gold/10 px-2 py-0.5 tracking-[0.12em] text-spade-gold-light">
          {room.code}
        </span>
        <span>{room.players} · {room.timer}</span>
      </div>

      <Button
        variant={room.open ? 'primary' : 'secondary'}
        disabled={!room.open}
        onClick={onJoin}
        className="w-full"
      >
        {room.open ? 'Join' : 'Full'}
      </Button>
    </article>
  )
}
