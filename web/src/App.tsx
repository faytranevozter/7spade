import { Navigate, NavLink, Route, Routes, useLocation } from "react-router";
import { AuthPage } from "./pages/AuthPage";
import { OAuthCallbackPage } from "./pages/OAuthCallbackPage";
import { RegisterPage } from "./pages/RegisterPage";
import { GamePage } from "./pages/GamePage";
import { HistoryPage } from "./pages/HistoryPage";
import { LobbyPage } from "./pages/LobbyPage";
import { ResultsPage } from "./pages/ResultsPage";
import { useAuth } from "./hooks/useAuth";

function App() {
  const { pathname } = useLocation();
  const { isAuthenticated } = useAuth();
  const hideHeader = pathname === "/auth" || pathname === "/register" || pathname === "/login" || pathname === "/auth/callback";

  return (
    <div className="min-h-svh bg-spade-bg text-spade-cream">
      {!hideHeader ? (
        <header className="sticky top-0 z-20 border-b border-spade-green-light/25 bg-spade-bg/95 px-4 py-3 backdrop-blur sm:px-6">
          <div className="mx-auto flex max-w-7xl items-center justify-between gap-3">
            <NavLink to="/auth" className="flex items-center gap-3">
              <span className="grid size-11 place-items-center rounded-spade-lg bg-linear-to-br from-spade-gold to-spade-gold-light text-2xl text-[#1a0e00] shadow-spade-card">
                ♠
              </span>
              <span className="block text-xl font-medium tracking-normal">
                Seven Spade
              </span>
            </NavLink>
            {isAuthenticated ? (
              <nav aria-label="Primary navigation" className="flex items-center gap-2 text-sm">
                <NavLink
                  to="/lobby"
                  className={({ isActive }) =>
                    `rounded-spade-pill px-3 py-2 ${isActive ? "bg-spade-green-mid text-spade-gold" : "text-spade-gray-2 hover:text-spade-cream"}`
                  }
                >
                  Lobby
                </NavLink>
                <NavLink
                  to="/history"
                  className={({ isActive }) =>
                    `rounded-spade-pill px-3 py-2 ${isActive ? "bg-spade-green-mid text-spade-gold" : "text-spade-gray-2 hover:text-spade-cream"}`
                  }
                >
                  My Games
                </NavLink>
              </nav>
            ) : null}
          </div>
        </header>
      ) : null}

      <main>
        <Routes>
          <Route index element={<Navigate replace to="/auth" />} />
          <Route path="/auth" element={<AuthPage />} />
          <Route path="/auth/callback" element={<OAuthCallbackPage />} />
          <Route path="/login" element={<Navigate replace to="/auth" />} />
          <Route path="/register" element={<RegisterPage />} />
          <Route path="/lobby" element={<LobbyPage />} />
          <Route path="/game/:roomId" element={<GamePage />} />
          <Route path="/results/:roomId" element={<ResultsPage />} />
          <Route path="/history" element={<HistoryPage />} />
          <Route path="*" element={<Navigate replace to="/auth" />} />
        </Routes>
      </main>
    </div>
  );
}

export default App;
