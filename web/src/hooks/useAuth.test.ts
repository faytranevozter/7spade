import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useAuth } from './useAuth';

describe('useAuth', () => {
  const mockToken = 'mock-jwt-token';

  beforeEach(() => {
    // Clear localStorage before each test
    localStorage.clear();
  });

  afterEach(() => {
    localStorage.clear();
  });

  it('should initialize with no token', () => {
    const { result } = renderHook(() => useAuth());

    expect(result.current.token).toBeNull();
    expect(result.current.isAuthenticated).toBe(false);
  });

  it('should initialize with token from localStorage if present', () => {
    localStorage.setItem('seven_spade_auth_token', mockToken);

    const { result } = renderHook(() => useAuth());

    expect(result.current.token).toBe(mockToken);
    expect(result.current.isAuthenticated).toBe(true);
  });

  it('should update token and localStorage when login is called', () => {
    const { result } = renderHook(() => useAuth());

    act(() => {
      result.current.login(mockToken);
    });

    expect(result.current.token).toBe(mockToken);
    expect(result.current.isAuthenticated).toBe(true);
    expect(localStorage.getItem('seven_spade_auth_token')).toBe(mockToken);
  });

  it('should clear token and localStorage when logout is called', () => {
    localStorage.setItem('seven_spade_auth_token', mockToken);
    const { result } = renderHook(() => useAuth());

    expect(result.current.isAuthenticated).toBe(true);

    act(() => {
      result.current.logout();
    });

    expect(result.current.token).toBeNull();
    expect(result.current.isAuthenticated).toBe(false);
    expect(localStorage.getItem('seven_spade_auth_token')).toBeNull();
  });

  it('should return false for isAuthenticated when token is empty string', () => {
    const { result } = renderHook(() => useAuth());

    act(() => {
      result.current.login('');
    });

    expect(result.current.isAuthenticated).toBe(false);
  });
});
