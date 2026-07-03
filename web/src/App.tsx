import { type ReactNode, useEffect, useState } from "react";
import { Navigate, NavLink, Route, Routes, useLocation, useNavigate } from "react-router";
import { AuthPage } from "./pages/AuthPage";
import { OAuthCallbackPage } from "./pages/OAuthCallbackPage";
import { RegisterPage } from "./pages/RegisterPage";
import { ForgotPasswordPage } from "./pages/ForgotPasswordPage";
import { ResetPasswordPage } from "./pages/ResetPasswordPage";
import { VerifyEmailPage } from "./pages/VerifyEmailPage";
import { VerifyEmailBanner } from "./components/VerifyEmailBanner";
import { GamePage } from "./pages/GamePage";
import { HistoryPage } from "./pages/HistoryPage";
import { LeaderboardPage } from "./pages/LeaderboardPage";
import { LobbyPage } from "./pages/LobbyPage";
import { ProfilePage } from "./pages/ProfilePage";
import { MyProfilePage } from "./pages/MyProfilePage";
import { ResultsPage } from "./pages/ResultsPage";
import { SpectatorPage } from "./pages/SpectatorPage";
import { ReplayPage } from "./pages/ReplayPage";
import { WaitingRoomPage } from "./pages/WaitingRoomPage";
import { AuthProvider } from "./hooks/AuthProvider";
import { useAuth } from "./hooks/useAuth";
import { ActiveRoomProvider } from "./hooks/ActiveRoomProvider";
import { ActiveGameButton } from "./components/ActiveGameButton";
import { PiPProvider, usePiPContext } from "./hooks/PiPProvider";
import { useSound } from "./hooks/useSound";
import { useMotion } from "./hooks/useMotion";
import { type MotionSpeed } from "./game/motion";
import { deleteLogout } from "./api/auth";
import { getFriends } from "./api/friends";
import { decodeJwtClaims } from "./auth/claims";

// Header control labels for the card-animation speed cycle button.
const MOTION_LABELS: Record<MotionSpeed, string> = {
  off: "Off",
  slow: "Slow",
  normal: "Normal",
  fast: "Fast",
};
const MOTION_ICONS: Record<MotionSpeed, string> = {
  off: "⏸",
  slow: "🐢",
  normal: "🎬",
  fast: "⚡",
};

// RedirectIfAuthenticated keeps logged-in users off the login/register pages.
// Visiting them (via the Back button or a typed URL) bounces to the lobby.
// It renders immediately (no loading gate) so logged-out visitors never see a
// flash; once the boot-time silent refresh resolves a session, isAuthenticated
// flips and the redirect fires.
function RedirectIfAuthenticated({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth();
  if (isAuthenticated) {
    return <Navigate replace to="/lobby" />;
  }
  return children;
}

// AuthLoadingScreen is shown briefly while the provider attempts a silent token
// refresh on boot (new tab/window with a valid refresh cookie but no in-tab
// access token), but only on protected routes — public routes render right away.
function AuthLoadingScreen() {
  return (
    <div className="grid min-h-svh place-items-center bg-spade-bg text-spade-gray-2">
      <span className="font-mono text-xs uppercase tracking-[0.22em] text-spade-gold-light">Loading…</span>
    </div>
  );
}

// Public routes don't require a session, so they must never be held behind the
// boot-time refresh gate (that would flash a loading screen to logged-out
// visitors, and reset/verify links must work while signed out).
const PUBLIC_PATH_PREFIXES = ["/auth", "/register", "/login", "/forgot-password", "/reset-password", "/verify-email"];

function isPublicPath(pathname: string): boolean {
  return PUBLIC_PATH_PREFIXES.some((p) => pathname === p || pathname.startsWith(p + "/"));
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
  const { token, isAuthenticated, isLoading, logout } = useAuth();
  const { muted, supported: soundSupported, toggleMuted } = useSound();
  const { speed: motionSpeed, cycle: cycleMotion } = useMotion();
  const pip = usePiPContext();
  const isGameRoute = pathname.startsWith("/game/");
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

  const navClass = ({ isActive }: { isActive: boolean }) =>
    `relative inline-flex min-h-9 items-center justify-center rounded-spade-pill border px-3 py-1.5 text-xs font-medium transition sm:min-h-10 sm:px-4 sm:py-2 sm:text-sm ${
      isActive
        ? "border-spade-gold-light bg-spade-gold text-[#1a0e00] shadow-[0_0_24px_rgb(201_146_43_/_24%)]"
        : "border-spade-cream/10 bg-spade-bg/45 text-spade-gray-2 hover:border-spade-gold/45 hover:bg-spade-green/45 hover:text-spade-cream"
    }`;

  const utilityClass =
    "inline-flex min-h-9 items-center justify-center rounded-spade-pill border border-spade-cream/10 bg-spade-bg/45 px-3 py-1.5 text-xs font-medium text-spade-gray-2 transition hover:border-spade-gold/45 hover:bg-spade-green/45 hover:text-spade-cream sm:min-h-10 sm:py-2 sm:text-sm";

  // While the boot-time silent refresh is in flight, hold off rendering
  // protected routes so per-page guards don't redirect a valid (cookie-backed)
  // session to login. Public routes (auth/register/recovery) render immediately.
  if (isLoading && !isPublicPath(pathname)) {
    return <AuthLoadingScreen />;
  }

  return (
    <div className="min-h-svh bg-spade-bg text-spade-cream">
      {!hideHeader ? (
        <header className="sticky top-0 z-20 border-b border-spade-gold/15 bg-[#07130d]/90 px-3 py-2 shadow-[0_18px_60px_rgb(0_0_0_/_28%)] backdrop-blur-xl sm:px-6 sm:py-3">
          <div className="mx-auto flex max-w-7xl flex-col gap-2 rounded-spade-xl border border-spade-cream/10 bg-spade-green/20 px-3 py-2 shadow-spade-card sm:flex-row sm:items-center sm:justify-between sm:gap-3 sm:px-4 sm:py-3">
            <NavLink to="/lobby" className="group flex items-center gap-3">
              <img src="/logo.png" alt="Seven Spade" className="size-10 transition group-hover:scale-105 sm:size-12" />
              <span className="grid gap-0.5">
                <span className="text-lg font-medium leading-none tracking-tight text-spade-cream sm:text-xl">Seven Spade</span>
                <span className="font-mono text-[10px] uppercase tracking-[0.22em] text-spade-gold-light">Live card room</span>
              </span>
            </NavLink>
            {isAuthenticated ? (
              <div className="flex flex-wrap items-center gap-2 sm:justify-end">
                <nav aria-label="Primary navigation" className="flex flex-wrap items-center gap-1.5 rounded-spade-pill border border-spade-gold/15 bg-[#06110b]/55 p-1 sm:gap-2">
                  <NavLink to="/lobby" className={navClass}>
                    Lobby
                    {incomingRequests > 0 ? (
                      <span
                        aria-label={`${incomingRequests} friend requests`}
                        className="absolute -right-1 -top-1 grid min-w-5 place-items-center rounded-full border border-[#1a0e00]/20 bg-spade-gold-light px-1 text-[10px] font-bold text-[#1a0e00]"
                      >
                        {incomingRequests}
                      </span>
                    ) : null}
                  </NavLink>
                  <NavLink to="/history" className={navClass}>My Games</NavLink>
                  <NavLink to="/leaderboard" className={navClass}>Leaderboard</NavLink>
                  <NavLink to="/me" className={navClass}>Profile</NavLink>
                </nav>
                <div className="flex items-center gap-2 rounded-spade-pill border border-spade-cream/10 bg-[#06110b]/55 p-1">
                  <button
                    type="button"
                    onClick={cycleMotion}
                    aria-label={`Card animations: ${MOTION_LABELS[motionSpeed]}`}
                    title={`Card animations: ${MOTION_LABELS[motionSpeed]} (click to change)`}
                    className={utilityClass}
                  >
                    {MOTION_ICONS[motionSpeed]}
                  </button>
                  <button
                    type="button"
                    onClick={toggleMuted}
                    aria-label={muted ? "Unmute sound" : "Mute sound"}
                    aria-pressed={muted}
                    title={soundSupported ? (muted ? "Unmute sound" : "Mute sound") : "Sound not supported"}
                    className={utilityClass}
                  >
                    {muted ? "🔇" : "🔊"}
                  </button>
                  {isGameRoute && pip.isSupported ? (
                    <button
                      type="button"
                      onClick={pip.enabled ? pip.disable : pip.enable}
                      aria-label={pip.enabled ? "Disable Picture-in-Picture" : "Enable Picture-in-Picture"}
                      aria-pressed={pip.enabled}
                      title={pip.enabled ? "PiP: On (mini board stays open)" : "PiP: Off (click to pop out mini board)"}
                      className={`${utilityClass} ${pip.enabled ? '!border-spade-gold/60 !bg-spade-gold/15 !text-spade-gold-light' : ''}`}
                    >
                      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" className="h-4 w-4">
                        <path d="M2 4.5A2.5 2.5 0 0 1 4.5 2h11A2.5 2.5 0 0 1 18 4.5v11a2.5 2.5 0 0 1-2.5 2.5h-11A2.5 2.5 0 0 1 2 15.5v-11ZM10.5 10a1 1 0 0 0-1 1v3.5a1 1 0 0 0 1 1H16a1 1 0 0 0 1-1V11a1 1 0 0 0-1-1h-5.5Z" />
                      </svg>
                    </button>
                  ) : null}
                  <button type="button" onClick={handleSignOut} className={`${utilityClass} hover:border-spade-red/50 hover:text-spade-cream`}>
                    Sign out
                  </button>
                </div>
              </div>
            ) : null}
          </div>
        </header>
      ) : null}

      {!hideHeader ? <VerifyEmailBanner /> : null}

      <main>
        <Routes>
          <Route index element={<Navigate replace to="/auth" />} />
          <Route path="/auth" element={<RedirectIfAuthenticated><AuthPage /></RedirectIfAuthenticated>} />
          <Route path="/auth/callback" element={<OAuthCallbackPage />} />
          <Route path="/auth/callback/:provider" element={<OAuthCallbackPage />} />
          <Route path="/login" element={<Navigate replace to="/auth" />} />
          <Route path="/register" element={<RedirectIfAuthenticated><RegisterPage /></RedirectIfAuthenticated>} />
          <Route path="/forgot-password" element={<RedirectIfAuthenticated><ForgotPasswordPage /></RedirectIfAuthenticated>} />
          <Route path="/reset-password" element={<ResetPasswordPage />} />
          <Route path="/verify-email" element={<VerifyEmailPage />} />
          <Route path="/lobby" element={<LobbyPage />} />
          <Route path="/room/:roomId" element={<WaitingRoomPage />} />
          <Route path="/game/:roomId" element={<GamePage />} />
          <Route path="/results/:roomId" element={<ResultsPage />} />
          <Route path="/history" element={<HistoryPage />} />
          <Route path="/leaderboard" element={<LeaderboardPage />} />
          <Route path="/players/:id" element={<ProfilePage />} />
          <Route path="/me" element={<MyProfilePage />} />
          <Route path="/watch/:roomId" element={<SpectatorPage />} />
          <Route path="/replay/:gameId" element={<ReplayPage />} />
          <Route path="*" element={<Navigate replace to="/auth" />} />
        </Routes>
      </main>
      {isAuthenticated ? <ActiveGameButton /> : null}
    </div>
  );
}

function App() {
  return (
    <AuthProvider>
      <ActiveRoomProvider>
        <PiPProvider>
          <AppShell />
        </PiPProvider>
      </ActiveRoomProvider>
    </AuthProvider>
  );
}

export default App;
