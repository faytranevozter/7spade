import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import type { ReactNode } from 'react';
import { AuthProvider } from './AuthProvider';
import { useAuth } from './useAuth';

describe('useAuth', () => {
  const mockToken = 'mock-jwt-token';

  // useAuth now reads from AuthProvider context, so each hook render is wrapped
  // in the provider. The provider seeds its initial token from sessionStorage.
  const wrapper = ({ children }: { children: ReactNode }) => <AuthProvider>{children}</AuthProvider>;

  beforeEach(() => {
    // The provider persists the access token in sessionStorage (not localStorage)
    // so it survives a same-tab refresh but not a new tab/window.
    sessionStorage.clear();
  });

  afterEach(() => {
    sessionStorage.clear();
  });

  it('should initialize with no token', () => {
    const { result } = renderHook(() => useAuth(), { wrapper });

    expect(result.current.token).toBeNull();
    expect(result.current.isAuthenticated).toBe(false);
  });

  it('should initialize with token from sessionStorage if present', () => {
    sessionStorage.setItem('seven_spade_auth_token', mockToken);

    const { result } = renderHook(() => useAuth(), { wrapper });

    expect(result.current.token).toBe(mockToken);
    expect(result.current.isAuthenticated).toBe(true);
  });

  it('should update token and sessionStorage when login is called', () => {
    const { result } = renderHook(() => useAuth(), { wrapper });

    act(() => {
      result.current.login(mockToken);
    });

    expect(result.current.token).toBe(mockToken);
    expect(result.current.isAuthenticated).toBe(true);
    expect(sessionStorage.getItem('seven_spade_auth_token')).toBe(mockToken);
  });

  it('should clear token and sessionStorage when logout is called', () => {
    sessionStorage.setItem('seven_spade_auth_token', mockToken);
    const { result } = renderHook(() => useAuth(), { wrapper });

    expect(result.current.isAuthenticated).toBe(true);

    act(() => {
      result.current.logout();
    });

    expect(result.current.token).toBeNull();
    expect(result.current.isAuthenticated).toBe(false);
    expect(sessionStorage.getItem('seven_spade_auth_token')).toBeNull();
  });

  it('should return false for isAuthenticated when token is empty string', () => {
    const { result } = renderHook(() => useAuth(), { wrapper });

    act(() => {
      result.current.login('');
    });

    expect(result.current.isAuthenticated).toBe(false);
  });
});
