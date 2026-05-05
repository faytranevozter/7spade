import { useEffect, useState } from 'react'

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'
const WS_HEALTH_URL = import.meta.env.VITE_WS_HEALTH_URL ?? 'http://localhost:8081'

type ServiceState = 'checking' | 'ok' | 'degraded' | 'offline'

type ServiceStatus = {
  label: string
  url: string
  state: ServiceState
  details: string
}

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
    <main className="min-h-svh bg-[radial-gradient(circle_at_top,#235c36_0%,#0d1a12_42%,#08100b_100%)] px-4 py-6 text-spade-cream sm:px-6 lg:px-10">
      <section className="mx-auto grid max-w-6xl gap-5 lg:grid-cols-[1.1fr_0.9fr]">
        <div className="rounded-spade-xl border border-spade-green-light/35 bg-spade-green/80 p-5 shadow-spade-table backdrop-blur sm:p-7">
          <div className="mb-6 flex flex-wrap items-center gap-3">
            <div className="grid size-14 place-items-center rounded-spade-lg bg-gradient-to-br from-spade-gold to-spade-gold-light text-3xl text-spade-bg shadow-spade-card">
              ♠
            </div>
            <div>
              <p className="font-mono text-[11px] uppercase tracking-[0.18em] text-spade-gold">Monorepo online</p>
              <h1 className="text-3xl font-medium tracking-[-0.04em] text-spade-cream sm:text-5xl">Seven Spade</h1>
            </div>
            <span className="ml-auto rounded-spade-pill border border-spade-green-light bg-spade-green-mid px-3 py-1 font-mono text-[11px] uppercase tracking-[0.14em] text-spade-gold">
              scaffold
            </span>
          </div>

          <div className="rounded-spade-xl border border-spade-cream/10 bg-spade-bg/50 p-4">
            <div className="mb-4 flex items-center justify-between gap-3">
              <div>
                <h2 className="text-sm font-medium uppercase tracking-[0.14em] text-spade-gray-2">Local table check</h2>
                <p className="mt-1 text-sm text-spade-gray-3">React, Go services, PostgreSQL, and Redis are wired through Docker Compose.</p>
              </div>
              <div className="rounded-full border border-spade-gold/40 px-3 py-1 font-mono text-xs text-spade-gold">4P</div>
            </div>

            <div className="grid gap-2">
              {['♠', '♥', '♦', '♣'].map((suit) => (
                <div key={suit} className="grid grid-cols-[2rem_repeat(7,minmax(2.25rem,1fr))] items-center gap-2">
                  <div className="text-center text-xl">{suit}</div>
                  {['A', 'K', 'Q', '7', '8', '9', '10'].map((rank, index) => (
                    <div
                      key={`${suit}-${rank}`}
                      className={`grid aspect-[5/7] place-items-center rounded-md border text-sm font-bold shadow-spade-card ${
                        index === 3
                          ? 'border-spade-gold bg-spade-white text-spade-black'
                          : 'border-dashed border-spade-cream/20 bg-spade-green-mid/50 text-spade-cream/30'
                      }`}
                    >
                      {index === 3 ? rank : ''}
                    </div>
                  ))}
                </div>
              ))}
            </div>
          </div>
        </div>

        <aside className="grid gap-4">
          <div className="rounded-spade-xl border border-spade-cream/10 bg-spade-bg/80 p-5 shadow-spade-table">
            <h2 className="font-mono text-xs uppercase tracking-[0.16em] text-spade-gray-3">Service health</h2>
            <div className="mt-4 grid gap-3">
              {statuses.map((service) => (
                <article key={service.label} className="rounded-spade-lg border border-spade-cream/10 bg-spade-green/60 p-4">
                  <div className="flex items-center justify-between gap-3">
                    <h3 className="text-lg font-medium text-spade-cream">{service.label}</h3>
                    <span className={`status-badge status-${service.state}`}>{service.state}</span>
                  </div>
                  <p className="mt-2 font-mono text-xs text-spade-gray-3">{service.details}</p>
                </article>
              ))}
            </div>
          </div>

          <div className="rounded-spade-xl border border-spade-gold/30 bg-spade-cream p-5 text-spade-black shadow-spade-card">
            <p className="font-mono text-[11px] uppercase tracking-[0.16em] text-spade-gray-4">Room preview</p>
            <div className="mt-4 flex items-end justify-between gap-4">
              <div>
                <h2 className="text-2xl font-medium tracking-[-0.03em]">Docker Table</h2>
                <p className="mt-1 text-sm text-spade-gray-4">Waiting for game logic slices.</p>
              </div>
              <div className="rounded-spade-md bg-spade-green px-3 py-2 text-center text-spade-cream">
                <div className="font-mono text-lg">0/4</div>
                <div className="text-[10px] uppercase tracking-[0.12em] text-spade-gray-2">players</div>
              </div>
            </div>
          </div>
        </aside>
      </section>
    </main>
  )
}

export default App
