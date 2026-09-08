import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import { unequipSkin, type UserSkinsResponse } from '../api/skins'
import { setEquippedSkins } from '../hooks/useEquippedSkins'
import { MyProfilePage } from './MyProfilePage'

vi.mock('../hooks/useAuth', () => ({
  useAuth: () => ({ token: 'token', isAuthenticated: true, login: vi.fn() }),
}))
vi.mock('../auth/claims', () => ({
  decodeJwtClaims: () => ({ userId: 'current-user', displayName: 'Player', isGuest: false }),
}))
vi.mock('../api/auth', () => ({
  AuthApiError: class AuthApiError extends Error {},
  getMe: vi.fn().mockResolvedValue(null),
  cancelDeletion: vi.fn(),
  deleteAccount: vi.fn(),
  updateDisplayName: vi.fn(),
}))
vi.mock('../api/stats', () => ({
  getMyStats: vi.fn().mockResolvedValue(null),
  getRatingHistory: vi.fn().mockResolvedValue({ events: [] }),
}))
vi.mock('../api/achievements', () => ({
  getUserAchievements: vi.fn().mockResolvedValue({ earned: [], catalog: [] }),
}))
vi.mock('../api/skins', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/skins')>()
  return {
    ...actual,
    getMySkins: vi.fn().mockResolvedValue({ owned: [], equipped: [] }),
    getSkinCatalog: vi.fn().mockResolvedValue({ skins: [] }),
    equipSkin: vi.fn(),
    unequipSkin: vi.fn(),
  }
})
vi.mock('../hooks/useEquippedSkins', () => ({ setEquippedSkins: vi.fn() }))
vi.mock('../components/ProfileView', () => ({
  ProfileView: ({ tabs }: { tabs: Array<{ id: string; panel: React.ReactNode }> }) => (
    <>{tabs.find((tab) => tab.id === 'cosmetics')?.panel}</>
  ),
}))
vi.mock('../components/SkinPicker', () => ({
  SkinPicker: ({ onUnequip }: { onUnequip: (type: 'avatar_frame') => void }) => (
    <button type="button" onClick={() => onUnequip('avatar_frame')}>Use default</button>
  ),
}))

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

test('publishes successful unequip results so mounted identity consumers use the default', async () => {
  const response: UserSkinsResponse = { owned: [], equipped: [] }
  vi.mocked(unequipSkin).mockResolvedValue(response)

  render(<MemoryRouter><MyProfilePage /></MemoryRouter>)
  fireEvent.click(screen.getByRole('button', { name: 'Use default' }))

  await waitFor(() => expect(setEquippedSkins).toHaveBeenCalledWith('current-user', response.equipped))
})
