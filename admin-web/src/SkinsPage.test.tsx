import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import { getSkins } from './api/skins'
import { useAuth } from './hooks/useAuth'
import { SkinsPage } from './pages/SkinsPage'

vi.mock('./api/skins', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api/skins')>()),
  getSkins: vi.fn(),
}))
vi.mock('./hooks/useAuth', () => ({ useAuth: vi.fn() }))

afterEach(() => {
  cleanup()
  vi.resetAllMocks()
})

test('paginates the skin catalog and resets to the first page after filtering', async () => {
  vi.mocked(useAuth).mockReturnValue({
    token: 'token',
    admin: { permissions: [] },
  } as unknown as ReturnType<typeof useAuth>)
  vi.mocked(getSkins).mockResolvedValue({
    skins: Array.from({ length: 13 }, (_, index) => ({
      id: `skin-${index + 1}`,
      skin_type: 'avatar_frame',
      name: `Skin ${index + 1}`,
      description: '',
      asset_key: '',
      asset_url: '',
      is_starter: false,
      display_order: index,
      enabled: true,
      catalog_visible: true,
      unlock_rules_locked: false,
      unlock_rules: [],
      revisions: [],
    })),
  })
  render(
    <MemoryRouter>
      <SkinsPage />
    </MemoryRouter>,
  )
  expect(
    await screen.findByRole('link', { name: 'Open Skin 1' }),
  ).toBeInTheDocument()
  expect(
    screen.queryByRole('link', { name: 'Open Skin 13' }),
  ).not.toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Next' }))
  expect(
    await screen.findByRole('link', { name: 'Open Skin 13' }),
  ).toBeInTheDocument()
  fireEvent.change(screen.getByPlaceholderText('Name, description, or ID'), {
    target: { value: 'Skin 2' },
  })
  expect(
    await screen.findByRole('link', { name: 'Open Skin 2' }),
  ).toBeInTheDocument()
  expect(
    screen.queryByRole('link', { name: 'Open Skin 13' }),
  ).not.toBeInTheDocument()
})
