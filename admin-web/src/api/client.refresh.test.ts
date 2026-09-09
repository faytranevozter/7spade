import { afterEach, expect, it, vi } from 'vitest'
import { apiBlob, apiResponse, setSessionRecovery } from './client'

afterEach(() => {
  setSessionRecovery(null)
  vi.unstubAllGlobals()
})

it('coalesces overlapping and late 401s, and uses the rotated token on future requests', async () => {
  let finishRefresh!: (token: string) => void
  let finishLate!: (response: Response) => void
  const recover = vi.fn(
    () =>
      new Promise<string>((resolve) => {
        finishRefresh = resolve
      }),
  )
  setSessionRecovery(recover)
  const fetchMock = vi.fn(async (url: string, init: RequestInit) => {
    const auth = new Headers(init.headers).get('Authorization')
    if (auth === 'Bearer old') {
      if (url.endsWith('/late'))
        return new Promise<Response>((resolve) => {
          finishLate = resolve
        })
      return new Response(null, { status: 401 })
    }
    return new Response('{}')
  })
  vi.stubGlobal('fetch', fetchMock)
  const init = { headers: { Authorization: 'Bearer old' } }
  const first = apiResponse('/first', init)
  const second = apiResponse('/second', init)
  const late = apiBlob('/late', init)
  await vi.waitFor(() => expect(recover).toHaveBeenCalledTimes(1))
  finishRefresh('new')
  await Promise.all([first, second])
  finishLate(new Response(null, { status: 401 }))
  await late
  await apiResponse('/future', init)
  expect(recover).toHaveBeenCalledTimes(1)
  expect(fetchMock).toHaveBeenCalledTimes(7)
})

it('expires on retry 401 without looping', async () => {
  const expire = vi.fn()
  const recover = vi.fn(async () => 'new')
  setSessionRecovery(recover, undefined, expire)
  const fetchMock = vi.fn(async () => new Response(null, { status: 401 }))
  vi.stubGlobal('fetch', fetchMock)
  await expect(
    apiResponse('/mutation', {
      method: 'POST',
      body: '{}',
      headers: { Authorization: 'Bearer old' },
    }),
  ).rejects.toMatchObject({ status: 401 })
  expect(fetchMock).toHaveBeenCalledTimes(2)
  expect(recover).toHaveBeenCalledTimes(1)
  expect(expire).toHaveBeenCalledTimes(1)
})

it('does not retry after logout while refresh is pending', async () => {
  let finish!: (token: string) => void
  const recover = vi.fn(
    () =>
      new Promise<string>((resolve) => {
        finish = resolve
      }),
  )
  setSessionRecovery(recover)
  const fetchMock = vi.fn(async () => new Response(null, { status: 401 }))
  vi.stubGlobal('fetch', fetchMock)
  const result = apiResponse('/data', {
    headers: { Authorization: 'Bearer old' },
  })
  const assertion = expect(result).rejects.toMatchObject({ status: 401 })
  await vi.waitFor(() => expect(recover).toHaveBeenCalledTimes(1))
  setSessionRecovery(null)
  finish('new')
  await assertion
  expect(fetchMock).toHaveBeenCalledTimes(1)
})
