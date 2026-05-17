import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthApiError, getOAuthStartUrl, postLogin, postOAuthCallback, postRefresh, postRegister } from './auth'

describe('auth API', () => {
  beforeEach(() => {
    global.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('registers with credentials included for refresh cookie', async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse({ jwt: 'mock-jwt-token' }, 201))

    const result = await postRegister('test@example.com', 'password123', 'Test User')

    expect(result).toEqual({ jwt: 'mock-jwt-token' })
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/register',
      expect.objectContaining({
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: 'test@example.com', password: 'password123', display_name: 'Test User' }),
      }),
    )
  })

  it('logs in with credentials included for refresh cookie', async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse({ jwt: 'mock-jwt-token' }))

    const result = await postLogin('test@example.com', 'password123')

    expect(result).toEqual({ jwt: 'mock-jwt-token' })
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/login',
      expect.objectContaining({ method: 'POST', credentials: 'include' }),
    )
  })

  it('refreshes using only the HttpOnly cookie', async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse({ jwt: 'new-jwt-token' }))

    const result = await postRefresh()

    expect(result).toEqual({ jwt: 'new-jwt-token' })
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/refresh',
      expect.objectContaining({ method: 'POST', credentials: 'include' }),
    )
  })

  it('fetches OAuth start URL from backend', async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse({ url: 'https://provider.example/auth', state: 'state-1' }))

    const result = await getOAuthStartUrl('github')

    expect(result).toEqual({ url: 'https://provider.example/auth', state: 'state-1' })
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/auth/github/url',
      expect.objectContaining({ credentials: 'include' }),
    )
  })

  it('posts OAuth callback and normalises access_token to jwt', async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse({ access_token: 'oauth-jwt' }))

    const result = await postOAuthCallback('github', 'code-1', 'state-1')

    expect(result).toEqual({ jwt: 'oauth-jwt' })
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/auth/github/callback',
      expect.objectContaining({
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code: 'code-1', state: 'state-1' }),
      }),
    )
  })

  it('throws AuthApiError on failure', async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(jsonResponse({ error: 'Email already registered' }, 409))

    await expect(postRegister('test@example.com', 'password123', 'Test User')).rejects.toThrow(AuthApiError)
  })
})

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}
