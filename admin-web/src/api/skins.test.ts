import { afterEach, expect, test, vi } from 'vitest'
import { getSkin } from './skins'

afterEach(() => vi.restoreAllMocks())

test('requests one skin with authentication and normalizes nullable collections', async () => {
  const fetchMock = vi
    .spyOn(globalThis, 'fetch')
    .mockResolvedValue(
      new Response(
        JSON.stringify({ id: 'skin-1', unlock_rules: null, revisions: null }),
      ),
    )
  expect(await getSkin('token', 'skin/1')).toEqual({
    id: 'skin-1',
    asset_url: '',
    is_starter: false,
    unlock_rules: [],
    revisions: [],
  })
  expect(fetchMock).toHaveBeenCalledExactlyOnceWith(
    expect.stringMatching(/\/skins\/skin%2F1$/),
    expect.objectContaining({ credentials: 'include' }),
  )
  const headers = new Headers(fetchMock.mock.calls[0][1]?.headers)
  expect(headers.get('Authorization')).toBe('Bearer token')
})

test('preserves enriched skin detail fields', async () => {
  const skin = {
    id: 'skin-1',
    asset_url: 'https://cdn.example/skin.png',
    is_starter: true,
    unlock_rules_locked: true,
    unlock_rules: [{ rule_type: 'minimum_level', minimum_level: 2 }],
    revisions: [{ id: 'revision-1', version: 1, enabled: false }],
  }
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(
    new Response(JSON.stringify(skin)),
  )
  expect(await getSkin('token', 'skin-1')).toEqual(skin)
})

test.each([403, 404, 500])(
  'propagates detail HTTP %s without requesting the catalog',
  async (status) => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: 'Detail unavailable' }), {
        status,
      }),
    )
    await expect(getSkin('token', 'missing')).rejects.toMatchObject({
      status,
      message: 'Detail unavailable',
    })
    expect(fetchMock).toHaveBeenCalledTimes(1)
  },
)
