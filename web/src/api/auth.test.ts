import { describe, it, expect, vi, beforeEach } from 'vitest';
import { postGuest, AuthApiError } from './auth';

describe('postGuest', () => {
  const mockFetch = vi.fn();

  beforeEach(() => {
    vi.stubGlobal('fetch', mockFetch);
    mockFetch.mockClear();
  });

  it('should call the correct endpoint with correct payload', async () => {
    const mockToken = 'mock-jwt-token';
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ token: mockToken }),
    });

    await postGuest('TestUser');

    expect(mockFetch).toHaveBeenCalledWith(
      'http://localhost:8080/guest',
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ display_name: 'TestUser' }),
      }
    );
  });

  it('should return token from response', async () => {
    const mockToken = 'mock-jwt-token';
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ token: mockToken }),
    });

    const result = await postGuest('TestUser');

    expect(result.token).toBe(mockToken);
  });

  it('should throw AuthApiError for empty display name', async () => {
    await expect(postGuest('')).rejects.toThrow(AuthApiError);
    await expect(postGuest('')).rejects.toThrow('Display name is required');
    
    // Should not call fetch
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it('should throw AuthApiError for display name longer than 50 characters', async () => {
    const longName = 'a'.repeat(51);
    
    await expect(postGuest(longName)).rejects.toThrow(AuthApiError);
    await expect(postGuest(longName)).rejects.toThrow('Display name must be 50 characters or less');
    
    // Should not call fetch
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it('should throw AuthApiError when API returns 400', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 400,
      json: async () => ({ error: 'Display name is required' }),
    });

    try {
      await postGuest('TestUser');
      expect.fail('Should have thrown an error');
    } catch (err) {
      expect(err).toBeInstanceOf(AuthApiError);
      expect((err as AuthApiError).statusCode).toBe(400);
      expect((err as AuthApiError).message).toBe('Display name is required');
    }
  });

  it('should throw AuthApiError when API returns 500', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: async () => ({ error: 'Internal server error' }),
    });

    try {
      await postGuest('TestUser');
      expect.fail('Should have thrown an error');
    } catch (err) {
      expect(err).toBeInstanceOf(AuthApiError);
      expect((err as AuthApiError).statusCode).toBe(500);
    }
  });

  it('should handle API error without error field', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 503,
      json: async () => ({}),
    });

    try {
      await postGuest('TestUser');
      expect.fail('Should have thrown an error');
    } catch (err) {
      expect(err).toBeInstanceOf(AuthApiError);
      expect((err as AuthApiError).statusCode).toBe(503);
      expect((err as AuthApiError).message).toContain('Request failed with status 503');
    }
  });

  it('should handle network errors', async () => {
    mockFetch.mockRejectedValueOnce(new Error('Network error'));

    await expect(postGuest('TestUser')).rejects.toThrow('Network error');
  });
});
