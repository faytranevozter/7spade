import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { getApplicationControls, type ApplicationControls } from '../api/applicationControls'
import { useApplicationControls } from './useApplicationControls'

vi.mock('../api/applicationControls', () => ({ getApplicationControls: vi.fn() }))
const enabled: ApplicationControls = { new_registrations: true, guest_access: true, room_creation: true, quick_play: true }
const disabled: ApplicationControls = { new_registrations: false, guest_access: false, room_creation: false, quick_play: false }

afterEach(() => { cleanup(); vi.resetAllMocks() })

test('loads on mount, refreshes on focus, preserves flags on failure and recovers', async () => {
  vi.mocked(getApplicationControls).mockResolvedValueOnce(enabled).mockResolvedValueOnce(disabled).mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(enabled)
  const { result } = renderHook(useApplicationControls)
  await waitFor(() => expect(result.current).toEqual(enabled))
  await act(async () => window.dispatchEvent(new Event('focus')))
  expect(result.current).toEqual(disabled)
  await act(async () => window.dispatchEvent(new Event('focus')))
  expect(result.current).toEqual(disabled)
  await act(async () => window.dispatchEvent(new Event('focus')))
  expect(result.current).toEqual(enabled)
})

test('ignores older responses and removes the focus listener on unmount', async () => {
  let resolveOld!: (controls: ApplicationControls) => void
  let resolvePending!: (controls: ApplicationControls) => void
  vi.mocked(getApplicationControls)
    .mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve }))
    .mockResolvedValueOnce(disabled)
    .mockImplementationOnce(() => new Promise((resolve) => { resolvePending = resolve }))
  const { result, unmount } = renderHook(useApplicationControls)
  await act(async () => window.dispatchEvent(new Event('focus')))
  await act(async () => resolveOld(enabled))
  expect(result.current).toEqual(disabled)
  await act(async () => window.dispatchEvent(new Event('focus')))
  unmount()
  await act(async () => resolvePending(enabled))
  window.dispatchEvent(new Event('focus'))
  expect(getApplicationControls).toHaveBeenCalledTimes(3)
  expect(result.current).toEqual(disabled)
})
