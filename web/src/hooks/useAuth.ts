import { useState, useEffect } from 'react';

const AUTH_TOKEN_KEY = 'seven_spade_auth_token';
const REFRESH_TOKEN_KEY = 'seven_spade_refresh_token';

export interface UseAuthReturn {
  token: string | null;
  refreshToken: string | null;
  isAuthenticated: boolean;
  login: (token: string, refreshToken?: string) => void;
  logout: () => void;
  updateToken: (newToken: string) => void;
}

/**
 * Custom hook for managing authentication state
 * Stores JWT token and refresh token in localStorage and provides auth utilities
 */
export function useAuth(): UseAuthReturn {
  const [token, setToken] = useState<string | null>(() => {
    // Initialize from localStorage on mount
    if (typeof window !== 'undefined') {
      return localStorage.getItem(AUTH_TOKEN_KEY);
    }
    return null;
  });

  const [refreshToken, setRefreshToken] = useState<string | null>(() => {
    // Initialize from localStorage on mount
    if (typeof window !== 'undefined') {
      return localStorage.getItem(REFRESH_TOKEN_KEY);
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

  useEffect(() => {
    // Sync refresh token to localStorage whenever it changes
    if (refreshToken) {
      localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken);
    } else {
      localStorage.removeItem(REFRESH_TOKEN_KEY);
    }
  }, [refreshToken]);

  const login = (newToken: string, newRefreshToken?: string) => {
    setToken(newToken);
    if (newRefreshToken) {
      setRefreshToken(newRefreshToken);
    }
  };

  const logout = () => {
    setToken(null);
    setRefreshToken(null);
  };

  const updateToken = (newToken: string) => {
    setToken(newToken);
  };

  const isAuthenticated = token !== null && token.length > 0;

  return {
    token,
    refreshToken,
    isAuthenticated,
    login,
    logout,
    updateToken,
  };
}
