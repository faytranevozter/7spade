import { afterEach, describe, expect, it, vi } from 'vitest'
import { apiBlob, apiResponse, setSessionRecovery } from './client'

afterEach(() => {
  setSessionRecovery(null)
  vi.restoreAllMocks()
})

describe('session recovery', () => {
  it('refreshes and retries an authenticated JSON request once', async () => {
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response(null, { status: 401 }))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ enabled: true }), { status: 200 }),
      )
    const recover = vi.fn().mockResolvedValue('replacement-token')
    setSessionRecovery(recover)

    await expect(
      apiResponse('/settings', {
        headers: { Authorization: 'Bearer expired-token' },
      }),
    ).resolves.toEqual({ enabled: true })
    expect(recover).toHaveBeenCalledOnce()
    expect(
      new Headers(fetchMock.mock.calls[1][1]?.headers).get('Authorization'),
    ).toBe('Bearer replacement-token')
  })

  it('refreshes and retries authenticated blob requests', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response(null, { status: 401 }))
      .mockResolvedValueOnce(new Response('audit export', { status: 200 }))
    setSessionRecovery(() => Promise.resolve('replacement-token'))

    await expect(
      apiBlob('/audit/export', {
        headers: { Authorization: 'Bearer expired-token' },
      }),
    ).resolves.toBeInstanceOf(Blob)
  })

  it('does not recursively recover unauthenticated requests', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(null, { status: 401 }),
    )
    const recover = vi.fn()
    setSessionRecovery(recover)

    await expect(apiResponse('/auth/refresh')).rejects.toMatchObject({
      status: 401,
    })
    expect(recover).not.toHaveBeenCalled()
  })
})
