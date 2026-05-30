import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { AUTH_TOKEN_KEY, AuthContext, type UseAuthReturn } from './useAuth';

// AuthProvider holds the access token in shared React state so every consumer
// sees login/logout immediately (no remount needed). The token is mirrored to
// sessionStorage so it survives a same-tab refresh; the refresh token stays in
// an HttpOnly cookie managed by the backend.
export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => {
    if (typeof window !== 'undefined') {
      return sessionStorage.getItem(AUTH_TOKEN_KEY);
    }
    return null;
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

  const value = useMemo<UseAuthReturn>(
    () => ({
      token,
      isAuthenticated: token !== null && token.length > 0,
      login,
      logout,
    }),
    [token, login, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
