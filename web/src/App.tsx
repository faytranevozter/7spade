import { Navigate, NavLink, Route, Routes } from "react-router";
import { AuthPage } from "./pages/AuthPage";
import { LoginPage } from "./pages/LoginPage";
import { RegisterPage } from "./pages/RegisterPage";
import { GamePage } from "./pages/GamePage";
import { HistoryPage } from "./pages/HistoryPage";
import { LobbyPage } from "./pages/LobbyPage";
import { ResultsPage } from "./pages/ResultsPage";

function App() {
  return (
    <div className="min-h-svh bg-spade-bg text-spade-cream">
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
        </div>
      </header>

      <main>
        <Routes>
          <Route index element={<Navigate replace to="/auth" />} />
          <Route path="/auth" element={<AuthPage />} />
          <Route path="/login" element={<LoginPage />} />
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
