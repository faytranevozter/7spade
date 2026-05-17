import { useState } from 'react';

const AUTH_TOKEN_KEY = 'seven_spade_auth_token';

export interface UseAuthReturn {
  token: string | null;
  isAuthenticated: boolean;
  login: (token: string) => void;
  logout: () => void;
  updateToken: (newToken: string) => void;
}

export function useAuth(): UseAuthReturn {
  // Access token lives in React state only (in-memory, not localStorage).
  // Refresh token is in an HttpOnly cookie managed entirely by the backend.
  const [token, setToken] = useState<string | null>(() => {
    // On first load try to restore from sessionStorage so the user isn't
    // kicked out on a hard refresh within the same browser tab session.
    if (typeof window !== 'undefined') {
      return sessionStorage.getItem(AUTH_TOKEN_KEY);
    }
    return null;
  });

  const login = (newToken: string) => {
    setToken(newToken);
    if (typeof window !== 'undefined') {
      sessionStorage.setItem(AUTH_TOKEN_KEY, newToken);
    }
  };

  const logout = () => {
    setToken(null);
    if (typeof window !== 'undefined') {
      sessionStorage.removeItem(AUTH_TOKEN_KEY);
    }
  };

  const updateToken = (newToken: string) => {
    setToken(newToken);
    if (typeof window !== 'undefined') {
      sessionStorage.setItem(AUTH_TOKEN_KEY, newToken);
    }
  };

  const isAuthenticated = token !== null && token.length > 0;

  return { token, isAuthenticated, login, logout, updateToken };
}
