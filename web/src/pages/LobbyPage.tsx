import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { RoomCard } from '../components/RoomCard'
import { SectionPanel } from '../components/SectionPanel'
import { rooms } from '../data/mockGame'

export function LobbyPage() {
  return (
    <SectionPanel title="Game lobby" eyebrow="Room creation + lobby" action={<Badge tone="waiting">3 waiting</Badge>}>
      <div className="grid gap-4 lg:grid-cols-[360px_minmax(0,1fr)]">
        <div className="grid gap-4">
          <div className="rounded-spade-lg border border-spade-green-light/25 bg-spade-bg/70 p-4">
            <h3 className="text-lg font-medium">Create room</h3>
            <div className="mt-4 grid gap-3">
              <label className="grid gap-1 text-xs text-spade-gray-2">
                Room name
                <input defaultValue="Meja Santai #1" className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 text-sm text-spade-black" />
              </label>
              <label className="grid gap-1 text-xs text-spade-gray-2">
                Visibility
                <select className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 text-sm text-spade-black">
                  <option>Public room</option>
                  <option>Private invite code</option>
                </select>
              </label>
              <label className="grid gap-1 text-xs text-spade-gray-2">
                Turn timer
                <select className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 text-sm text-spade-black">
                  <option>30 seconds</option>
                  <option>60 seconds</option>
                  <option>90 seconds</option>
                  <option>120 seconds</option>
                </select>
              </label>
              <Button>Create room</Button>
            </div>
          </div>
          <div className="rounded-spade-lg border border-spade-green-light/25 bg-spade-bg/70 p-4">
            <h3 className="text-lg font-medium">Join by code</h3>
            <div className="mt-4 flex gap-2">
              <input defaultValue="XKQP7" className="min-w-0 flex-1 rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 font-mono text-sm tracking-[0.08em] text-spade-black" />
              <Button>Join</Button>
            </div>
          </div>
        </div>

        <div className="grid content-start gap-2">
          {rooms.map((room) => (
            <RoomCard key={room.code} room={room} />
          ))}
        </div>
      </div>
    </SectionPanel>
  )
}
