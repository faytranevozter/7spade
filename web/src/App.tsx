import { Navigate, NavLink, Route, Routes } from "react-router";
import { AuthPage } from "./pages/AuthPage";
import { GameScenarioPage, type GameScenario } from "./pages/GameScenarioPage";
import { HistoryPage } from "./pages/HistoryPage";
import { LobbyPage } from "./pages/LobbyPage";
import { ResultsPage } from "./pages/ResultsPage";

const gameRoutes: Array<{
  path: string;
  label: string;
  scenario: GameScenario;
}> = [
  {
    path: "/mock/game/my-turn/playable",
    label: "Playable",
    scenario: "playable",
  },
  {
    path: "/mock/game/my-turn/start-seven",
    label: "Start 7",
    scenario: "start-seven",
  },
  {
    path: "/mock/game/my-turn/no-valid-move",
    label: "No Move",
    scenario: "no-valid-move",
  },
  {
    path: "/mock/game/my-turn/invalid-move",
    label: "Invalid",
    scenario: "invalid-move",
  },
  {
    path: "/mock/game/my-turn/timer-warning",
    label: "Timer",
    scenario: "timer-warning",
  },
  {
    path: "/mock/game/opponent-turn",
    label: "Opponent",
    scenario: "opponent-turn",
  },
  {
    path: "/mock/game/opponent-played-card",
    label: "Played",
    scenario: "opponent-played-card",
  },
  {
    path: "/mock/game/opponent-passed",
    label: "Passed",
    scenario: "opponent-passed",
  },
  {
    path: "/mock/game/disconnected-player-bot",
    label: "Bot",
    scenario: "disconnected-player-bot",
  },
  { path: "/mock/game/reconnect", label: "Reconnect", scenario: "reconnect" },
  {
    path: "/mock/game/round-ending",
    label: "Round End",
    scenario: "round-ending",
  },
];

const navRoutes = [
  { path: "/mock/auth", label: "Auth" },
  { path: "/mock/lobby", label: "Lobby" },
  { path: "/mock/lobby/private-join", label: "Private Join" },
  ...gameRoutes.map(({ path, label }) => ({ path, label })),
  { path: "/mock/results", label: "Results" },
  { path: "/mock/results/rematch-ready", label: "Rematch" },
  { path: "/mock/history", label: "History" },
];

function App() {
  return (
    <div className="min-h-svh bg-spade-bg text-spade-cream">
      <header className="sticky top-0 z-20 border-b border-spade-green-light/25 bg-spade-bg/95 px-4 py-3 backdrop-blur sm:px-6">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-3">
          <NavLink to="/mock/auth" className="flex items-center gap-3">
            <span className="grid size-11 place-items-center rounded-spade-lg bg-linear-to-br from-spade-gold to-spade-gold-light text-2xl text-[#1a0e00] shadow-spade-card">
              ♠
            </span>
            <span>
              <span className="block text-xl font-medium tracking-normal">
                Seven Spade
              </span>
              <span className="block font-mono text-[11px] uppercase tracking-[0.12em] text-spade-gray-3">
                Static React/Tailwind prototype - no backend calls
              </span>
            </span>
          </NavLink>

          <nav
            aria-label="Prototype scenes"
            className="flex max-w-full gap-1 overflow-x-auto rounded-spade-pill border border-spade-green-light/25 bg-spade-green/60 p-1"
          >
            {navRoutes.map((route) => (
              <NavLink
                key={route.path}
                to={route.path}
                className={({ isActive }) =>
                  `whitespace-nowrap rounded-spade-pill px-3 py-1.5 text-xs font-medium transition hover:bg-spade-green-light/35 hover:text-spade-cream ${
                    isActive
                      ? "bg-spade-gold text-[#1a0e00]"
                      : "text-spade-gray-2"
                  }`
                }
              >
                {route.label}
              </NavLink>
            ))}
          </nav>
        </div>
      </header>

      <main>
        <Routes>
          <Route index element={<Navigate replace to="/mock/auth" />} />
          <Route path="/mock/auth" element={<AuthPage />} />
          <Route path="/mock/lobby" element={<LobbyPage />} />
          <Route
            path="/mock/lobby/private-join"
            element={<LobbyPage privateJoin />}
          />
          {gameRoutes.map((route) => (
            <Route
              key={route.path}
              path={route.path}
              element={<GameScenarioPage scenario={route.scenario} />}
            />
          ))}
          <Route path="/mock/results" element={<ResultsPage />} />
          <Route
            path="/mock/results/rematch-ready"
            element={<ResultsPage rematchReady />}
          />
          <Route path="/mock/history" element={<HistoryPage />} />
          <Route path="*" element={<Navigate replace to="/mock/auth" />} />
        </Routes>
      </main>
    </div>
  );
}

export default App;
