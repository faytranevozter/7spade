import { type FormEvent, useState } from 'react'
import { useNavigate } from 'react-router'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { RoomCard } from '../components/RoomCard'
import { SceneShell } from '../components/SceneShell'
import type { Room } from '../types'

const rooms: Room[] = [
  { name: 'Meja Santai #1', code: 'XKQP7', players: '3 / 4', status: 'Waiting to start', timer: '60s', open: true },
  { name: 'Pro Room', code: 'PR7A2', players: '1 / 4', status: 'Public room', timer: '30s', open: true },
  { name: 'Friday Night Game', code: 'FNG44', players: '4 / 4', status: 'In progress', timer: '90s', open: false },
]

export function LobbyPage() {
  const navigate = useNavigate()
  const [visibility, setVisibility] = useState('public')
  const [timer, setTimer] = useState(60)
  const [inviteCode, setInviteCode] = useState('XKQP7')

  const handleCreateRoom = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    navigate(`/game/new-${visibility}-${timer}`)
  }

  const handleJoinRoom = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    navigate(`/game/${inviteCode.trim() || 'XKQP7'}`)
  }

  const openRoomCount = rooms.filter((room) => room.open).length

  return (
    <SceneShell title="Game lobby" eyebrow="Room creation + lobby" action={<Badge tone="waiting">{`${openRoomCount} waiting`}</Badge>}>
      <div className="grid gap-4 lg:grid-cols-[360px_minmax(0,1fr)]">
        <div className="grid content-start gap-4">
          <form onSubmit={handleCreateRoom} className="rounded-spade-lg border border-spade-green-light/25 bg-spade-bg/70 p-4">
            <h3 className="text-lg font-medium">Create room</h3>
            <div className="mt-4 grid gap-3">
              <label className="grid gap-1 text-xs text-spade-gray-2">
                Visibility
                <select
                  value={visibility}
                  onChange={(event) => setVisibility(event.target.value)}
                  className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 text-sm text-spade-black"
                >
                  <option value="public">Public room</option>
                  <option value="private">Private invite code</option>
                </select>
              </label>
              <label className="grid gap-1 text-xs text-spade-gray-2">
                Turn timer
                <select
                  value={timer}
                  onChange={(event) => setTimer(Number(event.target.value))}
                  className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 text-sm text-spade-black"
                >
                  <option value={30}>30 seconds</option>
                  <option value={60}>60 seconds</option>
                  <option value={90}>90 seconds</option>
                  <option value={120}>120 seconds</option>
                </select>
              </label>
              <Button type="submit">Create room</Button>
            </div>
          </form>

          <form onSubmit={handleJoinRoom} className="rounded-spade-lg border border-spade-green-light/25 bg-spade-bg/70 p-4">
            <h3 className="text-lg font-medium">Join private room</h3>
            <div className="mt-4 grid gap-3">
              <label className="grid gap-1 text-xs text-spade-gray-2">
                Invite code
                <input
                  value={inviteCode}
                  onChange={(event) => setInviteCode(event.target.value.toUpperCase())}
                  className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 font-mono text-sm tracking-[0.08em] text-spade-black"
                />
              </label>
              <Button type="submit">Join with code</Button>
            </div>
          </form>
        </div>

        <div className="grid content-start gap-2">
          {rooms.map((room) => (
            <RoomCard
              key={room.code}
              room={room}
              onJoin={() => navigate(`/game/${room.code}`)}
            />
          ))}
        </div>
      </div>
    </SceneShell>
  )
}
