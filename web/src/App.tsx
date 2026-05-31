import { type ReactNode, useEffect, useState } from "react";
import { Navigate, NavLink, Route, Routes, useLocation, useNavigate } from "react-router";
import { AuthPage } from "./pages/AuthPage";
import { OAuthCallbackPage } from "./pages/OAuthCallbackPage";
import { RegisterPage } from "./pages/RegisterPage";
import { GamePage } from "./pages/GamePage";
import { HistoryPage } from "./pages/HistoryPage";
import { LeaderboardPage } from "./pages/LeaderboardPage";
import { LobbyPage } from "./pages/LobbyPage";
import { ProfilePage } from "./pages/ProfilePage";
import { ResultsPage } from "./pages/ResultsPage";
import { SpectatorPage } from "./pages/SpectatorPage";
import { WaitingRoomPage } from "./pages/WaitingRoomPage";
import { AuthProvider } from "./hooks/AuthProvider";
import { useAuth } from "./hooks/useAuth";
import { useSound } from "./hooks/useSound";
import { deleteLogout } from "./api/auth";
import { getFriends } from "./api/friends";
import { decodeJwtClaims } from "./auth/claims";

// RedirectIfAuthenticated keeps logged-in users off the login/register pages.
// Visiting them (via the Back button or a typed URL) bounces to the lobby.
function RedirectIfAuthenticated({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth();
  if (isAuthenticated) {
    return <Navigate replace to="/lobby" />;
  }
  return children;
}

// useIncomingFriendRequests polls the friends list and returns the count of
// incoming pending requests, for the header badge. Skipped for guests / when
// signed out. Polls on a slow cadence since it's only a nudge.
function useIncomingFriendRequests(token: string | null, isAuthenticated: boolean): number {
  const [count, setCount] = useState(0);
  const enabled = isAuthenticated && !decodeJwtClaims(token).isGuest;

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    const load = () => {
      getFriends(token)
        .then((data) => {
          if (cancelled) return;
          setCount(data.friends.filter((f) => f.status === "incoming").length);
        })
        .catch(() => {
          // Non-fatal; the badge just won't update.
        });
    };
    load();
    const interval = window.setInterval(load, 15000);
    return () => {
      cancelled = true;
      window.clearInterval(interval);
    };
  }, [token, enabled]);

  // When disabled (guest / signed out), report zero without writing state in
  // the effect.
  return enabled ? count : 0;
}

function AppShell() {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const { token, isAuthenticated, logout } = useAuth();
  const { muted, supported: soundSupported, toggleMuted } = useSound();
  const incomingRequests = useIncomingFriendRequests(token, isAuthenticated);
  const hideHeader = pathname === "/auth" || pathname === "/register" || pathname === "/login" || pathname.startsWith("/auth/callback");

  const handleSignOut = () => {
    // Drop the local session and leave immediately so a slow or hanging request
    // can't strand the user on an authed page. The backend refresh-cookie clear
    // is best-effort and fired without blocking the UI.
    logout();
    navigate("/auth", { replace: true });
    void deleteLogout().catch(() => {
      // ignore — local logout above is what matters for the user.
    });
  };

  return (
    <div className="min-h-svh bg-spade-bg text-spade-cream">
      {!hideHeader ? (
        <header className="sticky top-0 z-20 border-b border-spade-green-light/25 bg-spade-bg/95 px-4 py-3 backdrop-blur sm:px-6">
          <div className="mx-auto flex max-w-7xl items-center justify-between gap-3">
            <NavLink to="/lobby" className="flex items-center gap-3">
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
                    `relative rounded-spade-pill px-3 py-2 ${isActive ? "bg-spade-green-mid text-spade-gold" : "text-spade-gray-2 hover:text-spade-cream"}`
                  }
                >
                  Lobby
                  {incomingRequests > 0 ? (
                    <span
                      aria-label={`${incomingRequests} friend requests`}
                      className="absolute -right-1 -top-1 grid min-w-4 place-items-center rounded-full bg-spade-gold px-1 text-[10px] font-bold text-[#1a0e00]"
                    >
                      {incomingRequests}
                    </span>
                  ) : null}
                </NavLink>
                <NavLink
                  to="/history"
                  className={({ isActive }) =>
                    `rounded-spade-pill px-3 py-2 ${isActive ? "bg-spade-green-mid text-spade-gold" : "text-spade-gray-2 hover:text-spade-cream"}`
                  }
                >
                  My Games
                </NavLink>
                <NavLink
                  to="/leaderboard"
                  className={({ isActive }) =>
                    `rounded-spade-pill px-3 py-2 ${isActive ? "bg-spade-green-mid text-spade-gold" : "text-spade-gray-2 hover:text-spade-cream"}`
                  }
                >
                  Leaderboard
                </NavLink>
                <button
                  type="button"
                  onClick={toggleMuted}
                  aria-label={muted ? "Unmute sound" : "Mute sound"}
                  aria-pressed={muted}
                  title={soundSupported ? (muted ? "Unmute sound" : "Mute sound") : "Sound not supported"}
                  className="rounded-spade-pill px-3 py-2 text-spade-gray-2 transition hover:text-spade-cream"
                >
                  {muted ? "🔇" : "🔊"}
                </button>
                <button
                  type="button"
                  onClick={handleSignOut}
                  className="rounded-spade-pill px-3 py-2 text-spade-gray-2 transition hover:text-spade-cream"
                >
                  Sign out
                </button>
              </nav>
            ) : null}
          </div>
        </header>
      ) : null}

      <main>
        <Routes>
          <Route index element={<Navigate replace to="/auth" />} />
          <Route path="/auth" element={<RedirectIfAuthenticated><AuthPage /></RedirectIfAuthenticated>} />
          <Route path="/auth/callback" element={<OAuthCallbackPage />} />
          <Route path="/auth/callback/:provider" element={<OAuthCallbackPage />} />
          <Route path="/login" element={<Navigate replace to="/auth" />} />
          <Route path="/register" element={<RedirectIfAuthenticated><RegisterPage /></RedirectIfAuthenticated>} />
          <Route path="/lobby" element={<LobbyPage />} />
          <Route path="/room/:roomId" element={<WaitingRoomPage />} />
          <Route path="/game/:roomId" element={<GamePage />} />
          <Route path="/results/:roomId" element={<ResultsPage />} />
          <Route path="/history" element={<HistoryPage />} />
          <Route path="/leaderboard" element={<LeaderboardPage />} />
          <Route path="/players/:id" element={<ProfilePage />} />
          <Route path="/watch/:roomId" element={<SpectatorPage />} />
          <Route path="*" element={<Navigate replace to="/auth" />} />
        </Routes>
      </main>
    </div>
  );
}

function App() {
  return (
    <AuthProvider>
      <AppShell />
    </AuthProvider>
  );
}

export default App;
