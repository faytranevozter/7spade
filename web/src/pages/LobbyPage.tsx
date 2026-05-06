import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { Modal } from '../components/Modal'
import { RoomCard } from '../components/RoomCard'
import { SceneShell } from '../components/SceneShell'
import { rooms } from '../data/mockGame'
import { Link } from 'react-router'

export function LobbyPage({ privateJoin = false }: { privateJoin?: boolean }) {
  return (
    <SceneShell title="Game lobby" eyebrow="Room creation + lobby" action={<Badge tone="waiting">3 waiting</Badge>}>
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
            <h3 className="text-lg font-medium">Join private room</h3>
            <div className="mt-4 flex flex-wrap gap-2">
              <label className="grid min-w-0 flex-1 gap-1 text-xs text-spade-gray-2">
                Invite code
                <input defaultValue="XKQP7" className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 font-mono text-sm tracking-[0.08em] text-spade-black" />
              </label>
              <div className="flex items-end gap-2">
                <Button>Join with code</Button>
              </div>
              <Link
                className="inline-flex min-h-9 items-center justify-center self-end rounded-spade-md border border-spade-gold px-4 py-2 text-sm font-medium text-spade-gold-light transition hover:bg-spade-gold/10"
                to="/mock/lobby/private-join"
              >
                Private
              </Link>
            </div>
          </div>
        </div>

        <div className="grid content-start gap-2">
          {rooms.map((room) => (
            <RoomCard key={room.code} room={room} />
          ))}
        </div>
      </div>

      {privateJoin ? (
      <div className="mt-6 grid gap-4">
        <Modal
          description="Private tables ask for the room password before the player joins the waiting list."
          eyebrow="Private room"
          footer={(
            <>
              <Button variant="secondary">Cancel</Button>
              <Button>Join private room</Button>
            </>
          )}
          title="Join private room"
        >
          <div className="grid gap-3">
            <label className="grid gap-1 text-xs text-spade-gray-2">
              Room code
              <input
                defaultValue="XKQP7"
                className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 font-mono text-sm tracking-[0.08em] text-spade-black"
              />
            </label>
            <label className="grid gap-1 text-xs text-spade-gray-2">
              Password
              <input
                defaultValue="seven"
                type="password"
                className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 text-sm text-spade-black"
              />
            </label>
          </div>
        </Modal>
      </div>
      ) : null}
    </SceneShell>
  )
}
