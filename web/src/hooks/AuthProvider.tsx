import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { AUTH_TOKEN_KEY, AuthContext, type UseAuthReturn } from './useAuth';
import { postRefresh } from '../api/auth';

// AuthProvider holds the access token in shared React state so every consumer
// sees login/logout immediately (no remount needed). The token is mirrored to
// sessionStorage so it survives a same-tab refresh; the refresh token stays in
// an HttpOnly cookie managed by the backend.
//
// sessionStorage is per-tab, so a brand-new tab/window starts with no access
// token even when the user has a valid session. To avoid bouncing such tabs to
// the login page, on boot (when there's no in-tab token) we attempt a silent
// refresh using the shared HttpOnly refresh cookie. `isLoading` stays true until
// that settles so route guards don't redirect prematurely.
export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => {
    if (typeof window !== 'undefined') {
      return sessionStorage.getItem(AUTH_TOKEN_KEY);
    }
    return null;
  });

  // Only attempt the boot-time silent refresh when we don't already have a
  // same-tab token. If we do, there's nothing to wait for.
  const [isLoading, setIsLoading] = useState<boolean>(() => {
    if (typeof window === 'undefined') return false;
    return sessionStorage.getItem(AUTH_TOKEN_KEY) === null;
  });

  const login = useCallback((newToken: string) => {
    setToken(newToken);
    if (typeof window !== 'undefined') {
      sessionStorage.setItem(AUTH_TOKEN_KEY, newToken);
    }
  }, []);

  const logout = useCallback(() => {
    setToken(null);
    if (typeof window !== 'undefined') {
      sessionStorage.removeItem(AUTH_TOKEN_KEY);
    }
  }, []);

  useEffect(() => {
    if (!isLoading) return;
    let cancelled = false;
    postRefresh()
      .then((res) => {
        if (cancelled) return;
        if (res.jwt) {
          login(res.jwt);
        }
      })
      .catch(() => {
        // No valid refresh cookie (or it expired): stay logged out. Route
        // guards will then send the user to the login page as before.
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [isLoading, login]);

  const value = useMemo<UseAuthReturn>(
    () => ({
      token,
      isAuthenticated: token !== null && token.length > 0,
      isLoading,
      login,
      logout,
    }),
    [token, isLoading, login, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
