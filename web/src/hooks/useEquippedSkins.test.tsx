import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { getUserSkins, type EquippedSkinDto } from '../api/skins'
import { setEquippedSkins, useEquippedSkins } from './useEquippedSkins'

vi.mock('../api/skins', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/skins')>()
  return { ...actual, getUserSkins: vi.fn() }
})

const frame: EquippedSkinDto = {
  skin_type: 'avatar_frame',
  skin_id: 'gold-frame',
  asset_key: 'frames/gold.svg',
}

afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.clearAllMocks()
})

test('deduplicates public loads and updates every mounted consumer when default is selected', async () => {
  const userID = 'mounted-user'
  vi.mocked(getUserSkins).mockResolvedValue({ owned: [], equipped: [frame] })

  const first = renderHook(() => useEquippedSkins(userID))
  const second = renderHook(() => useEquippedSkins(userID))

  await waitFor(() => expect(first.result.current).toEqual([frame]))
  expect(second.result.current).toEqual([frame])
  expect(getUserSkins).toHaveBeenCalledTimes(1)

  act(() => setEquippedSkins(userID, []))

  expect(first.result.current).toEqual([])
  expect(second.result.current).toEqual([])
})

test('revalidates a mounted public entry after the cache TTL', async () => {
  vi.useFakeTimers()
  const userID = 'ttl-user'
  vi.mocked(getUserSkins)
    .mockResolvedValueOnce({ owned: [], equipped: [frame] })
    .mockResolvedValueOnce({ owned: [], equipped: [] })

  const { result } = renderHook(() => useEquippedSkins(userID))
  await act(async () => { await vi.runAllTicks() })
  expect(result.current).toEqual([frame])

  await act(async () => { await vi.advanceTimersByTimeAsync(60_000) })

  expect(getUserSkins).toHaveBeenCalledTimes(2)
  expect(result.current).toEqual([])
})

test('loads the navigated player and prevents an older request overwriting a newer update', async () => {
  const firstUser = 'navigation-user-a'
  const secondUser = 'navigation-user-b'
  let resolveFirst!: (value: { owned: []; equipped: EquippedSkinDto[] }) => void
  vi.mocked(getUserSkins).mockImplementation((_token, userID) => {
    if (userID === firstUser) {
      return new Promise((resolve) => { resolveFirst = resolve })
    }
    return Promise.resolve({ owned: [], equipped: [] })
  })

  const { result, rerender } = renderHook(
    ({ userID }) => useEquippedSkins(userID),
    { initialProps: { userID: firstUser } },
  )

  act(() => setEquippedSkins(firstUser, [frame]))
  expect(result.current).toEqual([frame])
  resolveFirst({ owned: [], equipped: [] })
  await act(async () => { await Promise.resolve() })
  expect(result.current).toEqual([frame])

  rerender({ userID: secondUser })
  await waitFor(() => expect(getUserSkins).toHaveBeenCalledWith(null, secondUser))
  expect(result.current).toEqual([])
})
