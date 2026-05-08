import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { postRegister, postLogin, postRefresh, AuthApiError } from './auth'

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
