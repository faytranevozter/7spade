import { afterEach, expect, it, vi } from 'vitest'
import { apiResponse, setSessionRecovery } from './client'

afterEach(() => { setSessionRecovery(null); vi.unstubAllGlobals() })

it('expires once on failed refresh and never refreshes unauthenticated requests', async () => {
  const recover = vi.fn(async () => { throw new Error('refresh rejected') })
  const expire = vi.fn()
  setSessionRecovery(recover, undefined, expire)
  vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 401 })))
  await expect(apiResponse('/auth/refresh')).rejects.toMatchObject({ status: 401 })
  expect(recover).not.toHaveBeenCalled()
  const init = { headers: { Authorization: 'Bearer old' } }
  await Promise.all([
    expect(apiResponse('/one', init)).rejects.toThrow('refresh rejected'),
    expect(apiResponse('/two', init)).rejects.toThrow('refresh rejected'),
  ])
  expect(recover).toHaveBeenCalledTimes(1)
  expect(expire).toHaveBeenCalledTimes(1)
  await expect(apiResponse('/late', init)).rejects.toMatchObject({ status: 401 })
  expect(recover).toHaveBeenCalledTimes(1)
})
