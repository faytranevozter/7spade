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
import { createSkin } from './api/skins'
import { useAuth } from './hooks/useAuth'
import { SkinCreatePage } from './pages/SkinCreatePage'

vi.mock('./api/skins', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api/skins')>()),
  createSkin: vi.fn(),
}))
vi.mock('./hooks/useAuth', () => ({ useAuth: vi.fn() }))

afterEach(() => {
  cleanup()
  vi.resetAllMocks()
})

function Location() {
  return <output>{useLocation().pathname}</output>
}

test('creates a draft from a dedicated page and opens its record', async () => {
  vi.mocked(useAuth).mockReturnValue({ token: 'token' } as ReturnType<
    typeof useAuth
  >)
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
  render(
    <MemoryRouter initialEntries={['/skins/new']}>
      <Routes>
        <Route path="/skins/new" element={<SkinCreatePage />} />
        <Route path="/skins/:id" element={<Location />} />
      </Routes>
    </MemoryRouter>,
  )
  expect(screen.getByRole('button', { name: 'Create draft' })).toBeDisabled()
  fireEvent.change(screen.getByLabelText('New skin name'), {
    target: { value: 'Aurora Frame' },
  })
  fireEvent.change(screen.getByLabelText('New skin type'), {
    target: { value: 'avatar_frame' },
  })
  fireEvent.change(screen.getByLabelText('Creation reason'), {
    target: { value: 'new seasonal reward' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Create draft' }))
  await waitFor(() =>
    expect(createSkin).toHaveBeenCalledWith(
      'token',
      expect.objectContaining({
        name: 'Aurora Frame',
        skin_type: 'avatar_frame',
        reason: 'new seasonal reward',
      }),
    ),
  )
  expect(await screen.findByText('/skins/skin-1')).toBeInTheDocument()
})
