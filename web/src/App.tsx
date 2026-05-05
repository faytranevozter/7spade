import { useEffect, useState } from 'react'

type Suit = 'Spades' | 'Hearts' | 'Diamonds' | 'Clubs'

type ServiceState = 'checking' | 'ok' | 'degraded' | 'offline'

type Card = {
  rank: string
  suit: Suit
  playable?: boolean
  selected?: boolean
}

type ServiceStatus = {
  label: string
  url: string
  state: ServiceState
  details: string
}

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'
const WS_HEALTH_URL = import.meta.env.VITE_WS_HEALTH_URL ?? 'http://localhost:8081'

const suitSymbols: Record<Suit, string> = {
  Spades: '♠',
  Hearts: '♥',
  Diamonds: '♦',
  Clubs: '♣',
}

const suitTone: Record<Suit, string> = {
  Spades: 'text-[#1a1a1a]',
  Hearts: 'text-[#c0392b]',
  Diamonds: 'text-[#c0392b]',
  Clubs: 'text-[#1a1a1a]',
}

const suitRows: Array<{ suit: Suit; cards: Array<string | null>; closed?: boolean }> = [
  { suit: 'Hearts', cards: [null, null, '5', '6', '7', '8', '9', null, null] },
  { suit: 'Spades', cards: [null, null, null, '7', '8', null, null, null, null] },
  { suit: 'Diamonds', cards: [null, null, null, null, '7', '8', '9', '10', null] },
  { suit: 'Clubs', cards: [null, null, null, null, null, null, null, null, null], closed: true },
]

const hand: Card[] = [
  { rank: '4', suit: 'Hearts' },
  { rank: '6', suit: 'Spades', playable: true },
  { rank: '8', suit: 'Diamonds', selected: true },
  { rank: 'J', suit: 'Clubs' },
  { rank: 'A', suit: 'Spades' },
]

const rooms = [
  { name: 'Meja Santai #1', meta: '3 / 4 players · 60s timer · Waiting', open: true },
  { name: 'Pro Room', meta: '1 / 4 players · 30s timer · Public', open: true },
  { name: 'Friday Night Game', meta: '4 / 4 players · In progress', open: false },
]

const players = [
  { name: 'Fahrur', initials: 'FA', cards: '8 cards', penalties: '2 down', tone: 'bg-[#235c36]', active: true },
  { name: 'Budi', initials: 'BU', cards: '11 cards', penalties: '0 down', tone: 'bg-[#7a5010]' },
  { name: 'Santi', initials: 'SA', cards: '6 cards', penalties: '3 down', tone: 'bg-[#2a2a3a]' },
  { name: 'Rini', initials: 'RI', cards: '0 cards', penalties: 'winner', tone: 'bg-[#922b21]', winner: true },
]

const scores = [
  { rank: '1', player: 'Rini', penalty: 0, result: 'Winner' },
  { rank: '2', player: 'Fahrur (you)', penalty: 12, result: 'Shared table' },
  { rank: '3', player: 'Santi', penalty: 24, result: 'Finished' },
  { rank: '4', player: 'Budi', penalty: 52, result: 'Finished' },
]

const initialStatuses: ServiceStatus[] = [
  { label: 'HTTP API', url: API_URL, state: 'checking', details: 'Checking /health' },
  { label: 'Game WS', url: WS_HEALTH_URL, state: 'checking', details: 'Checking /health' },
]

async function readHealth(url: string): Promise<Pick<ServiceStatus, 'state' | 'details'>> {
  const response = await fetch(`${url}/health`)
  const body = await response.json()
  const dependencies = body.dependencies
    ? Object.entries(body.dependencies)
        .map(([name, state]) => `${name}: ${state}`)
        .join(' / ')
    : 'service only'

  if (!response.ok || body.status !== 'ok') {
    return { state: 'degraded', details: dependencies }
  }

  return { state: 'ok', details: dependencies }
}

function CardFace({ card, small = false }: { card: Card; small?: boolean }) {
  const isRed = card.suit === 'Hearts' || card.suit === 'Diamonds'
  const size = small ? 'h-19 w-13 rounded-[10px]' : 'h-25 w-17.5 rounded-[10px]'
  const lift = card.selected ? '-translate-y-3 ring-2 ring-[#c9922b]' : ''
  const playable = card.playable ? 'ring-2 ring-[#2d7a46]' : ''
  const label = `${card.rank} of ${card.suit}`

  return (
    <button
      type="button"
      aria-label={card.playable ? `Play ${label}` : label}
      data-state={card.playable ? 'playable' : card.selected ? 'selected' : 'idle'}
      className={`relative shrink-0 ${size} ${lift} ${playable} bg-[#fafaf8] text-left shadow-[0_2px_8px_rgba(0,0,0,0.18),0_0_0_1px_rgba(0,0,0,0.08)] transition duration-150 ease-[cubic-bezier(.34,1.56,.64,1)] hover:-translate-y-1.5 hover:shadow-[0_8px_24px_rgba(0,0,0,0.26),0_0_0_1px_rgba(0,0,0,0.12)]`}
    >
      <span className={`absolute left-2 top-1.5 flex flex-col leading-none ${isRed ? 'text-[#c0392b]' : 'text-[#1a1a1a]'}`}>
        <span className="text-sm font-bold">{card.rank}</span>
        <span className="text-xs">{suitSymbols[card.suit]}</span>
      </span>
      <span className={`absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 text-2xl ${isRed ? 'text-[#c0392b]' : 'text-[#1a1a1a]'}`}>
        {suitSymbols[card.suit]}
      </span>
    </button>
  )
}

function Badge({ children, tone }: { children: string; tone: 'waiting' | 'playing' | 'passed' | 'winner' }) {
  const tones = {
    waiting: 'border-[#c9922b]/30 bg-[#c9922b]/12 text-[#f5c842] before:bg-[#c9922b]',
    playing: 'border-[#2d7a46]/30 bg-[#2d7a46]/12 text-[#7bd696] before:bg-[#2d7a46]',
    passed: 'border-[#9c9589]/30 bg-[#9c9589]/14 text-[#d9d4c8] before:bg-[#9c9589]',
    winner: 'border-[#c9922b]/45 bg-[#c9922b]/18 text-[#f5c842] before:bg-[#f5c842]',
  }

  return (
    <span className={`inline-flex items-center gap-1.5 rounded-[20px] border px-3 py-1 text-[11px] font-medium before:block before:size-1.5 before:rounded-full ${tones[tone]}`}>
      {children}
    </span>
  )
}

function App() {
  const [statuses, setStatuses] = useState<ServiceStatus[]>(initialStatuses)

  useEffect(() => {
    const controller = new AbortController()

    Promise.all(
      initialStatuses.map(async (service) => {
        try {
          const result = await readHealth(service.url)
          return { ...service, ...result }
        } catch {
          return { ...service, state: 'offline' as const, details: 'not reachable from browser' }
        }
      }),
    ).then((results) => {
      if (!controller.signal.aborted) {
        setStatuses(results)
      }
    })

    return () => controller.abort()
  }, [])

  return (
    <div className="min-h-svh bg-[#0d1a12] px-4 py-5 text-[#f4ead5] sm:px-6 lg:px-8">
      <main className="mx-auto grid max-w-7xl gap-4 lg:grid-cols-[360px_minmax(0,1fr)]">
        <section className="rounded-[18px] border border-[#2d7a46]/35 bg-[#102316] p-5 shadow-2xl shadow-black/30">
          <div className="mb-6 flex items-center gap-4">
            <div className="grid size-14 place-items-center rounded-xl bg-linear-to-br from-[#c9922b] to-[#f5c842] text-3xl text-[#1a0e00]">
              ♠
            </div>
            <div>
              <p className="font-mono text-[11px] uppercase tracking-[0.18em] text-[#c9922b]">Fan Tan variant</p>
              <h1 className="text-3xl font-medium tracking-[-0.02em]">Seven Spade</h1>
              <p className="text-sm text-[#9c9589]">Real-time multiplayer card table</p>
            </div>
          </div>

          <div className="grid gap-3 rounded-xl border border-[#2d7a46]/30 bg-[#0d1a12]/70 p-4">
            <label className="grid gap-1 text-xs font-medium text-[#d9d4c8]">
              Display name
              <input
                defaultValue="Fahrur"
                className="rounded-lg border border-[#5a5550]/60 bg-[#f4ead5] px-3 py-2 text-sm text-[#1a1a18] outline-none transition focus:border-[#c9922b] focus:ring-4 focus:ring-[#c9922b]/15"
              />
            </label>
            <div className="grid grid-cols-2 gap-2">
              <button className="rounded-lg bg-[#c9922b] px-4 py-2.5 text-sm font-medium text-[#1a0e00] transition active:scale-95">
                Play as guest
              </button>
              <button className="rounded-lg border border-[#c9922b] px-4 py-2.5 text-sm font-medium text-[#f5c842] transition hover:bg-[#c9922b]/10">
                Sign in
              </button>
            </div>
          </div>

          <div className="mt-4 grid gap-3 rounded-xl border border-[#2d7a46]/30 bg-[#12301d] p-4">
            <h2 className="text-lg font-medium">Create room</h2>
            <label className="grid gap-1 text-xs text-[#d9d4c8]">
              Visibility
              <select className="rounded-lg border border-[#5a5550]/60 bg-[#f4ead5] px-3 py-2 text-sm text-[#1a1a18]">
                <option>Public room</option>
                <option>Private invite code</option>
              </select>
            </label>
            <label className="grid gap-1 text-xs text-[#d9d4c8]">
              Turn timer
              <select className="rounded-lg border border-[#5a5550]/60 bg-[#f4ead5] px-3 py-2 text-sm text-[#1a1a18]">
                <option>30 seconds</option>
                <option>60 seconds</option>
                <option>90 seconds</option>
                <option>120 seconds</option>
              </select>
            </label>
          </div>

          <section className="mt-4" aria-labelledby="rooms-heading">
            <div className="mb-2 flex items-center justify-between">
              <h2 id="rooms-heading" className="text-lg font-medium">Open public rooms</h2>
              <Badge tone="waiting">3 waiting</Badge>
            </div>
            <div className="grid gap-2">
              {rooms.map((room) => (
                <article key={room.name} className={`flex items-center gap-3 rounded-xl border border-[#f4ead5]/10 bg-[#f4ead5] p-3 text-[#1a1a18] transition hover:bg-[#f0ece3] ${room.open ? '' : 'opacity-55'}`}>
                  <span className={`size-2.5 rounded-full ${room.open ? 'bg-[#2d7a46]' : 'bg-[#9c9589]'}`} />
                  <div className="min-w-0 flex-1">
                    <h3 className="truncate text-sm font-medium">{room.name}</h3>
                    <p className="text-xs text-[#5a5550]">{room.meta}</p>
                  </div>
                  <button className="rounded-md bg-[#c9922b] px-3 py-1.5 text-xs font-medium text-[#1a0e00] disabled:bg-transparent disabled:text-[#5a5550]" disabled={!room.open}>
                    {room.open ? 'Join' : 'Full'}
                  </button>
                </article>
              ))}
            </div>
          </section>

          <section className="mt-4 rounded-xl border border-[#f4ead5]/10 bg-[#0d1a12]/70 p-4" aria-labelledby="service-health-heading">
            <div className="mb-3 flex items-center justify-between gap-3">
              <div>
                <h2 id="service-health-heading" className="text-lg font-medium">Service health</h2>
                <p className="text-xs text-[#9c9589]">Docker Compose local services</p>
              </div>
              <span className="rounded-[20px] border border-[#c9922b]/40 px-3 py-1 font-mono text-xs text-[#f5c842]">scaffold</span>
            </div>
            <div className="grid gap-2">
              {statuses.map((service) => (
                <article key={service.label} className="rounded-lg border border-[#f4ead5]/10 bg-[#12301d] p-3">
                  <div className="flex items-center justify-between gap-3">
                    <h3 className="text-sm font-medium">{service.label}</h3>
                    <span className={`status-badge status-${service.state}`}>{service.state}</span>
                  </div>
                  <p className="mt-1 font-mono text-[11px] text-[#d9d4c8]">{service.details}</p>
                </article>
              ))}
            </div>
          </section>
        </section>

        <section className="grid gap-4">
          <div className="rounded-[18px] border border-[#2d7a46]/35 bg-[#1a472a] p-4 shadow-2xl shadow-black/30">
            <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
              <div>
                <p className="font-mono text-xs uppercase tracking-[0.12em] text-[#c9922b]">Room XKQP7 · turn 14</p>
                <h2 className="text-2xl font-medium">Fahrur must play 6♠ or choose a penalty</h2>
              </div>
              <div className="flex items-center gap-2">
                <Badge tone="playing">Your turn</Badge>
                <span className="rounded-[20px] border border-[#f5c842]/40 bg-[#c9922b]/15 px-3 py-1 font-mono text-xs text-[#f5c842]">00:18</span>
              </div>
            </div>

            <div className="mb-5 h-2 overflow-hidden rounded-full bg-[#0d1a12]/70">
              <div className="h-full w-3/5 rounded-full bg-linear-to-r from-[#f5c842] to-[#c9922b]" />
            </div>

            <div className="mb-5 grid grid-cols-2 gap-3 md:grid-cols-4">
              {players.map((player) => (
                <article key={player.name} className="rounded-xl border border-[#f4ead5]/10 bg-[#0d1a12]/45 p-3 text-center">
                  <div className={`mx-auto grid size-11 place-items-center rounded-full border-2 ${player.active || player.winner ? 'border-[#c9922b]' : 'border-transparent'} ${player.tone} text-sm font-medium text-[#f4ead5]`}>
                    {player.initials}
                  </div>
                  <h3 className="mt-2 text-sm font-medium">{player.name}</h3>
                  <p className="font-mono text-[11px] text-[#d9d4c8]">{player.cards} · {player.penalties}</p>
                </article>
              ))}
            </div>

            <div role="region" aria-label="Seven Spade game board" className="rounded-[18px] bg-[#235c36] p-3 shadow-inner shadow-black/25">
              {suitRows.map((row) => (
                <div key={row.suit} aria-label={`${row.suit} suit sequence`} className="mb-2 flex items-center gap-2 last:mb-0">
                  <span className={`w-6 shrink-0 text-center text-lg ${row.suit === 'Hearts' || row.suit === 'Diamonds' ? 'text-[#e05c4a]' : 'text-[#d0cfc9]'}`}>
                    {suitSymbols[row.suit]}
                  </span>
                  <div className="grid flex-1 grid-cols-9 gap-1.5">
                    {row.cards.map((rank, index) => (
                      <div key={`${row.suit}-${index}`} className={`grid h-14 place-items-center rounded-md border border-dashed border-[#f4ead5]/18 text-[10px] text-[#f4ead5]/25 sm:h-17 ${rank ? `border-0 bg-[#fafaf8] text-sm font-bold shadow-lg shadow-black/25 ${suitTone[row.suit]}` : ''}`}>
                        {rank ?? '·'}
                      </div>
                    ))}
                  </div>
                  {row.closed ? <Badge tone="passed">Closed</Badge> : null}
                </div>
              ))}
            </div>

            <div className="mt-5 flex flex-wrap items-end justify-center gap-3">
              {hand.map((card) => <CardFace key={`${card.rank}-${card.suit}`} card={card} />)}
              <div className="h-25 w-17.5 rounded-[10px] bg-[#1a472a] bg-[repeating-linear-gradient(45deg,rgba(255,255,255,0.04)_0,rgba(255,255,255,0.04)_1px,transparent_1px,transparent_8px)] shadow-[0_2px_8px_rgba(0,0,0,0.18)]" aria-label="Face-down penalty pile" />
            </div>
          </div>

          <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_340px]">
            <section role="dialog" aria-labelledby="penalty-title" className="rounded-[18px] border border-[#c9922b]/35 bg-[#f4ead5] p-4 text-[#1a1a18] shadow-xl">
              <div className="mb-3 flex items-center justify-between gap-3">
                <div>
                  <p className="font-mono text-xs uppercase tracking-[0.12em] text-[#7a5010]">No legal move state</p>
                  <h2 id="penalty-title" className="text-lg font-medium">Choose a face-down penalty card</h2>
                </div>
                <Badge tone="waiting">Required</Badge>
              </div>
              <p className="mb-4 text-sm text-[#5a5550]">When no valid sequence card is available, the rules require selecting one card to place face-down.</p>
              <div className="flex gap-3">
                <CardFace card={{ rank: 'J', suit: 'Clubs' }} small />
                <CardFace card={{ rank: 'A', suit: 'Spades' }} small />
                <button className="rounded-lg bg-[#c9922b] px-4 py-2 text-sm font-medium text-[#1a0e00]">Place face-down</button>
              </div>
            </section>

            <section className="overflow-hidden rounded-[18px] border border-[#f4ead5]/12 bg-[#f4ead5] text-[#1a1a18] shadow-xl">
              <div className="flex items-center justify-between border-b border-black/10 bg-[#f0ece3] px-4 py-3">
                <h2 className="text-lg font-medium">Results</h2>
                <Badge tone="winner">Tie aware</Badge>
              </div>
              <table aria-label="Final scoreboard" className="w-full text-sm">
                <thead className="bg-[#f0ece3] text-left font-mono text-[10px] uppercase tracking-[0.06em] text-[#9c9589]">
                  <tr><th className="px-4 py-2">#</th><th>Player</th><th>Penalty</th><th>Result</th></tr>
                </thead>
                <tbody>
                  {scores.map((score) => (
                    <tr key={score.player} className="border-t border-black/10 odd:bg-[#c9922b]/5">
                      <td className="px-4 py-2 font-medium text-[#c9922b]">{score.rank}</td>
                      <td>{score.player}</td>
                      <td className="font-mono">{score.penalty}</td>
                      <td className="text-xs text-[#5a5550]">{score.result}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <div className="p-4">
                <button className="w-full rounded-lg border border-[#c9922b] px-4 py-2 text-sm font-medium text-[#7a5010] hover:bg-[#c9922b]/10">Offer rematch</button>
              </div>
            </section>
          </div>
        </section>
      </main>
    </div>
  )
}

export default App
