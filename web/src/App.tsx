import { AuthPage } from './pages/AuthPage'
import { FaceDownPage } from './pages/FaceDownPage'
import { GamePage } from './pages/GamePage'
import { HistoryPage } from './pages/HistoryPage'
import { LobbyPage } from './pages/LobbyPage'
import { ResultsPage } from './pages/ResultsPage'

const pages = [
  { id: 'auth', label: 'Auth', component: <AuthPage /> },
  { id: 'lobby', label: 'Lobby', component: <LobbyPage /> },
  { id: 'game', label: 'Game', component: <GamePage /> },
  { id: 'face-down', label: 'Face-down', component: <FaceDownPage /> },
  { id: 'results', label: 'Results', component: <ResultsPage /> },
  { id: 'history', label: 'History', component: <HistoryPage /> },
]

function App() {
  return (
    <div className="min-h-svh bg-spade-bg text-spade-cream">
      <header className="sticky top-0 z-20 border-b border-spade-green-light/25 bg-spade-bg/95 px-4 py-3 backdrop-blur sm:px-6">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-3">
          <a href="#auth" className="flex items-center gap-3">
            <span className="grid size-11 place-items-center rounded-spade-lg bg-gradient-to-br from-spade-gold to-spade-gold-light text-2xl text-[#1a0e00] shadow-spade-card">
              ♠
            </span>
            <span>
              <span className="block text-xl font-medium tracking-normal">Seven Spade</span>
              <span className="block font-mono text-[11px] uppercase tracking-[0.12em] text-spade-gray-3">
                Static React/Tailwind prototype
              </span>
            </span>
          </a>

          <nav aria-label="Prototype pages" className="flex max-w-full gap-1 overflow-x-auto rounded-spade-pill border border-spade-green-light/25 bg-spade-green/60 p-1">
            {pages.map((page) => (
              <a
                key={page.id}
                href={`#${page.id}`}
                className="rounded-spade-pill px-3 py-1.5 text-xs font-medium text-spade-gray-2 transition hover:bg-spade-green-light/35 hover:text-spade-cream"
              >
                {page.label}
              </a>
            ))}
          </nav>
        </div>
      </header>

      <main className="mx-auto grid max-w-7xl gap-5 px-4 py-5 sm:px-6 lg:py-6">
        <section className="rounded-spade-xl border border-spade-green-light/25 bg-[#102316] p-4 shadow-spade-table sm:p-5">
          <div className="flex flex-wrap items-end justify-between gap-4">
            <div>
              <p className="font-mono text-xs uppercase tracking-[0.12em] text-spade-gold">Fan Tan variant</p>
              <h1 className="mt-1 text-3xl font-medium tracking-normal sm:text-[32px]">Frontend design foundation</h1>
              <p className="mt-2 max-w-3xl text-sm leading-6 text-spade-gray-2">
                Static mock pages using local data only. These screens establish the reusable visual primitives that later auth,
                lobby, WebSocket, scoring, and history slices can connect to real backend state.
              </p>
            </div>
            <div className="flex flex-wrap gap-2">
              <span className="rounded-spade-pill border border-spade-gold/40 bg-spade-gold/15 px-3 py-1 font-mono text-xs text-spade-gold-light">
                Tailwind 4.2
              </span>
              <span className="rounded-spade-pill border border-spade-green-light/45 bg-spade-green-light/15 px-3 py-1 font-mono text-xs text-[#7bd696]">
                No backend calls
              </span>
            </div>
          </div>
        </section>

        {pages.map((page) => (
          <section key={page.id} id={page.id} className="scroll-mt-24">
            {page.component}
          </section>
        ))}
      </main>
    </div>
  )
}

export default App
