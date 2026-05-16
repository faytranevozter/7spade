import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { postRegister, postLogin, postRefresh, postTelegramAuth, AuthApiError, parseOAuthCallbackFragment, getOAuthStartUrl } from './auth'

describe('postRegister', () => {
  beforeEach(() => {
    global.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('should successfully register a user', async () => {
    const mockResponse = {
      jwt: 'mock-jwt-token',
      refresh_token: 'mock-refresh-token',
    }

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(mockResponse), {
        status: 201,
        headers: { 'Content-Type': 'application/json' },
      })
    )

    const result = await postRegister('test@example.com', 'password123', 'Test User')
    
    expect(result).toEqual(mockResponse)
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/register',
      expect.objectContaining({
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email: 'test@example.com',
          password: 'password123',
          display_name: 'Test User',
        }),
      })
    )
  })

  it('should throw AuthApiError on failure', async () => {
    const errorResponse = { error: 'Email already registered' }

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(errorResponse), {
        status: 409,
        headers: { 'Content-Type': 'application/json' },
      })
    )

    await expect(postRegister('test@example.com', 'password123', 'Test User')).rejects.toThrow(
      AuthApiError
    )
  })
})

describe('postLogin', () => {
  beforeEach(() => {
    global.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('should successfully login a user', async () => {
    const mockResponse = {
      jwt: 'mock-jwt-token',
      refresh_token: 'mock-refresh-token',
    }

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(mockResponse), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    )

    const result = await postLogin('test@example.com', 'password123')
    
    expect(result).toEqual(mockResponse)
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/login',
      expect.objectContaining({
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email: 'test@example.com',
          password: 'password123',
        }),
      })
    )
  })

  it('should throw AuthApiError on invalid credentials', async () => {
    const errorResponse = { error: 'Invalid email or password' }

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(errorResponse), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      })
    )

    await expect(postLogin('test@example.com', 'wrongpassword')).rejects.toThrow(AuthApiError)
  })
})

describe('postRefresh', () => {
  beforeEach(() => {
    global.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('should successfully refresh JWT', async () => {
    const mockResponse = {
      jwt: 'new-jwt-token',
    }

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(mockResponse), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    )

    const result = await postRefresh('mock-refresh-token')
    
    expect(result).toEqual(mockResponse)
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/refresh',
      expect.objectContaining({
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          refresh_token: 'mock-refresh-token',
        }),
      })
    )
  })

  it('should throw AuthApiError on invalid token', async () => {
    const errorResponse = { error: 'Invalid or expired refresh token' }

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(errorResponse), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      })
    )

    await expect(postRefresh('invalid-token')).rejects.toThrow(AuthApiError)
  })
})

describe('getOAuthStartUrl', () => {
  it('returns the API URL for the given provider', () => {
    expect(getOAuthStartUrl('google')).toBe('http://localhost:8080/auth/google')
    expect(getOAuthStartUrl('github')).toBe('http://localhost:8080/auth/github')
  })
})

describe('parseOAuthCallbackFragment', () => {
  it('parses a successful callback fragment', () => {
    const result = parseOAuthCallbackFragment('#provider=google&jwt=jwt-value&refresh_token=refresh-value')
    expect(result.provider).toBe('google')
    expect(result.jwt).toBe('jwt-value')
    expect(result.refreshToken).toBe('refresh-value')
    expect(result.error).toBeUndefined()
  })

  it('parses an error callback fragment', () => {
    const result = parseOAuthCallbackFragment('#provider=github&error=access_denied')
    expect(result.provider).toBe('github')
    expect(result.error).toBe('access_denied')
    expect(result.jwt).toBeUndefined()
    expect(result.refreshToken).toBeUndefined()
  })

  it('handles fragment without leading #', () => {
    const result = parseOAuthCallbackFragment('provider=google&jwt=abc')
    expect(result.provider).toBe('google')
    expect(result.jwt).toBe('abc')
  })

  it('returns empty fields for an empty fragment', () => {
    const result = parseOAuthCallbackFragment('')
    expect(result.provider).toBe('')
    expect(result.jwt).toBeUndefined()
    expect(result.refreshToken).toBeUndefined()
    expect(result.error).toBeUndefined()
  })
})

describe('postTelegramAuth', () => {
  beforeEach(() => {
    global.fetch = vi.fn()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('should post Telegram widget payload', async () => {
    const mockResponse = {
      jwt: 'telegram-jwt-token',
      refresh_token: 'telegram-refresh-token',
    }
    const payload = {
      id: 123,
      first_name: 'Ada',
      auth_date: 1710000000,
      hash: 'valid-hash',
    }

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(mockResponse), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    )

    const result = await postTelegramAuth(payload)

    expect(result).toEqual(mockResponse)
    expect(global.fetch).toHaveBeenCalledWith(
      'http://localhost:8080/auth/telegram',
      expect.objectContaining({
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
    )
  })

  it('should throw AuthApiError for invalid Telegram payload', async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ error: 'Invalid or expired Telegram payload' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      })
    )

    await expect(postTelegramAuth({ id: 123, auth_date: 1710000000, hash: 'bad-hash' })).rejects.toThrow(AuthApiError)
  })
})
