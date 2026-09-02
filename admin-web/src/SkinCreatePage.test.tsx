import '@testing-library/jest-dom/vitest'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import { getEvents } from './api/events'
import {
  createSkin,
  getAchievements,
  publishSkin,
  saveSkin,
  uploadSkinAsset,
} from './api/skins'
import { useAuth } from './hooks/useAuth'
import { SkinCreatePage } from './pages/SkinCreatePage'

vi.mock('./api/events', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api/events')>()),
  getEvents: vi.fn(),
}))
vi.mock('./api/skins', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api/skins')>()),
  createSkin: vi.fn(),
  getAchievements: vi.fn(),
  publishSkin: vi.fn(),
  saveSkin: vi.fn(),
  uploadSkinAsset: vi.fn(),
}))
vi.mock('./hooks/useAuth', () => ({ useAuth: vi.fn() }))

afterEach(() => {
  cleanup()
  vi.resetAllMocks()
})

function Location() {
  return <output>{useLocation().pathname}</output>
}

test('creates a skin with its required image and unlock rule', async () => {
  vi.mocked(useAuth).mockReturnValue({ token: 'token' } as ReturnType<
    typeof useAuth
  >)
  vi.mocked(getAchievements).mockResolvedValue({
    achievements: [{ id: 'first_win', name: 'First Win' }],
  })
  vi.mocked(getEvents).mockResolvedValue({ events: [] })
  vi.mocked(createSkin).mockResolvedValue({
    id: 'skin-1',
    skin_type: 'avatar_frame',
    name: 'Aurora Frame',
    description: '',
    asset_key: '',
    asset_url: '',
    is_starter: false,
    display_order: 0,
    enabled: false,
    catalog_visible: false,
    unlock_rules_locked: false,
    unlock_rules: [],
    revisions: [],
  })
  vi.mocked(uploadSkinAsset).mockResolvedValue({
    asset_key: 'skins/aurora.png',
    preview_url: 'https://example.test/aurora.png',
    content_type: 'image/png',
  })
  vi.mocked(publishSkin).mockResolvedValue({
    id: 'revision-1',
    version: 1,
    asset_key: 'skins/aurora.png',
    content_type: 'image/png',
    enabled: true,
  })
  vi.mocked(saveSkin).mockResolvedValue({
    id: 'skin-1',
    skin_type: 'avatar_frame',
    name: 'Aurora Frame',
    description: '',
    asset_key: 'skins/aurora.png',
    asset_url: '',
    is_starter: false,
    display_order: 0,
    enabled: false,
    catalog_visible: false,
    unlock_rules_locked: false,
    unlock_rules: [],
    revisions: [],
  })
  render(
    <MemoryRouter initialEntries={['/skins/new']}>
      <Routes>
        <Route path="/skins/new" element={<SkinCreatePage />} />
        <Route path="/skins/:id" element={<Location />} />
      </Routes>
    </MemoryRouter>,
  )
  expect(screen.getByRole('button', { name: 'Create skin' })).toBeDisabled()
  fireEvent.change(screen.getByLabelText('New skin name'), {
    target: { value: 'Aurora Frame' },
  })
  fireEvent.change(await screen.findByLabelText('Achievement'), {
    target: { value: 'first_win' },
  })
  fireEvent.change(screen.getByLabelText('Skin image'), {
    target: {
      files: [new File(['image'], 'aurora.png', { type: 'image/png' })],
    },
  })
  fireEvent.change(screen.getByLabelText('Creation reason'), {
    target: { value: 'new seasonal reward' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Create skin' }))
  await waitFor(() =>
    expect(createSkin).toHaveBeenCalledWith(
      'token',
      expect.objectContaining({
        name: 'Aurora Frame',
        unlock_rules: [
          expect.objectContaining({ achievement_id: 'first_win' }),
        ],
      }),
    ),
  )
  expect(uploadSkinAsset).toHaveBeenCalledWith(
    'token',
    'skin-1',
    expect.any(File),
  )
  expect(publishSkin).toHaveBeenCalledWith(
    'token',
    'skin-1',
    'skins/aurora.png',
    'image/png',
    'new seasonal reward',
  )
  expect(saveSkin).toHaveBeenCalledWith(
    'token',
    expect.objectContaining({ id: 'skin-1', asset_key: 'skins/aurora.png' }),
    'new seasonal reward',
    expect.any(Array),
  )
  expect(await screen.findByText('/skins/skin-1')).toBeInTheDocument()
})
