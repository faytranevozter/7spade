import { createContext, useContext } from 'react';

const AUTH_TOKEN_KEY = 'seven_spade_auth_token';

export { AUTH_TOKEN_KEY };

export interface UseAuthReturn {
  token: string | null;
  isAuthenticated: boolean;
  // True while the provider is attempting a silent token refresh on boot (a new
  // tab/window has no sessionStorage token but may have a valid refresh cookie).
  // Consumers should wait for this to settle before redirecting to login.
  isLoading: boolean;
  login: (token: string) => void;
  logout: () => void;
}

export const AuthContext = createContext<UseAuthReturn | null>(null);

export function useAuth(): UseAuthReturn {
  const context = useContext(AuthContext);
  if (context === null) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
