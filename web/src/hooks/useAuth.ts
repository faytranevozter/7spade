import { useState, useEffect } from 'react';

const AUTH_TOKEN_KEY = 'seven_spade_auth_token';

export interface UseAuthReturn {
  token: string | null;
  isAuthenticated: boolean;
  login: (token: string) => void;
  logout: () => void;
}

/**
 * Custom hook for managing authentication state
 * Stores JWT token in localStorage and provides auth utilities
 */
export function useAuth(): UseAuthReturn {
  const [token, setToken] = useState<string | null>(() => {
    // Initialize from localStorage on mount
    if (typeof window !== 'undefined') {
      return localStorage.getItem(AUTH_TOKEN_KEY);
    }
    return null;
  });

  useEffect(() => {
    // Sync token to localStorage whenever it changes
    if (token) {
      localStorage.setItem(AUTH_TOKEN_KEY, token);
    } else {
      localStorage.removeItem(AUTH_TOKEN_KEY);
    }
  }, [token]);

  const login = (newToken: string) => {
    setToken(newToken);
  };

  const logout = () => {
    setToken(null);
  };

  const isAuthenticated = token !== null && token.length > 0;

  return {
    token,
    isAuthenticated,
    login,
    logout,
  };
}
