import { StrictMode, useEffect } from 'react'
import { act, cleanup, render, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { AuthProvider } from './AuthProvider'
import { useAuth, type AuthState } from './useAuth'
import { apiResponse, setSessionRecovery } from '../api/client'

let auth: AuthState
const load = vi.fn()
function Consumer() {
  const state = useAuth()
  useEffect(() => { auth = state }, [state])
  useEffect(() => { if (state.token) load() }, [state.token])
  return <input defaultValue="draft" />
}
const admin = { id: '1', email: 'admin@example.com', display_name: 'Admin', status: 'active', permissions: [] }
const response = (token: string) => new Response(JSON.stringify({ admin, access_token: token }))

afterEach(() => {
  cleanup()
  setSessionRecovery(null)
  vi.unstubAllGlobals()
  load.mockClear()
})

it('ignores a boot refresh that finishes after logout', async () => {
  let finish!: (response: Response) => void
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    if (url.endsWith('/auth/refresh')) return new Promise<Response>((resolve) => { finish = resolve })
    return new Response(null, { status: 204 })
  }))
  render(<AuthProvider><Consumer /></AuthProvider>)
  await act(async () => { await auth.signOut() })
  await act(async () => { finish(response('late')) })
  expect(auth.admin).toBeNull()
  expect(auth.token).toBeNull()
  expect(auth.isLoading).toBe(false)
})

it('shares StrictMode boot refresh and silently recovers without token-driven reload', async () => {
  let refreshes = 0
  vi.stubGlobal('fetch', vi.fn(async (url: string, init: RequestInit) => {
    if (url.endsWith('/auth/refresh')) return response(++refreshes === 1 ? 'old' : 'new')
    return new Headers(init.headers).get('Authorization') === 'Bearer new'
      ? new Response('{}') : new Response(null, { status: 401 })
  }))
  const view = render(<StrictMode><AuthProvider><Consumer /></AuthProvider></StrictMode>)
  await waitFor(() => expect(auth.token).toBe('old'))
  const input = view.getByRole('textbox') as HTMLInputElement
  input.value = 'unsaved changes'
  await act(async () => { await apiResponse('/data', { headers: { Authorization: `Bearer ${auth.token}` } }) })
  expect(refreshes).toBe(2)
  expect(auth.token).toBe('old')
  expect(load).toHaveBeenCalledTimes(1)
  expect(input.value).toBe('unsaved changes')
})

it('does not resurrect the session when refresh resolves after logout', async () => {
  let finish!: (response: Response) => void
  let refreshes = 0
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    if (url.endsWith('/auth/refresh')) {
      if (++refreshes === 1) return response('old')
      return new Promise<Response>((resolve) => { finish = resolve })
    }
    return new Response(null, { status: 204 })
  }))
  render(<AuthProvider><Consumer /></AuthProvider>)
  await waitFor(() => expect(auth.token).toBe('old'))
  const pending = auth.refreshSession()
  const assertion = expect(pending).rejects.toThrow('session expired')
  await act(async () => { await auth.signOut() })
  await act(async () => { finish(response('new')); await assertion })
  expect(auth.token).toBeNull()
  expect(auth.admin).toBeNull()
})
