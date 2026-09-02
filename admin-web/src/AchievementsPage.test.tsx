import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import { getAchievements } from './api/achievements'
import { useAuth } from './hooks/useAuth'
import { AchievementsPage } from './pages/AchievementsPage'

vi.mock('./api/achievements', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api/achievements')>()),
  getAchievements: vi.fn(),
}))
vi.mock('./hooks/useAuth', () => ({ useAuth: vi.fn() }))
afterEach(() => {
  cleanup()
  vi.resetAllMocks()
})

test('paginates achievements and resets after searching', async () => {
  vi.mocked(useAuth).mockReturnValue({
    token: 'token',
    admin: { permissions: [] },
  } as unknown as ReturnType<typeof useAuth>)
  vi.mocked(getAchievements).mockResolvedValue({
    achievements: Array.from({ length: 13 }, (_, index) => ({
      id: `achievement_${index + 1}`,
      name: `Achievement ${index + 1}`,
      description: '',
      icon: 'trophy',
      display_order: index,
      enabled: true,
      rules: [],
      rules_locked: false,
    })),
  })
  render(
    <MemoryRouter>
      <AchievementsPage />
    </MemoryRouter>,
  )
  expect(
    await screen.findByRole('heading', { name: 'Achievement 1' }),
  ).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Next' }))
  expect(
    await screen.findByRole('heading', { name: 'Achievement 13' }),
  ).toBeInTheDocument()
  fireEvent.change(screen.getByPlaceholderText('Name, description, or ID'), {
    target: { value: 'Achievement 2' },
  })
  expect(
    await screen.findByRole('heading', { name: 'Achievement 2' }),
  ).toBeInTheDocument()
  expect(
    screen.queryByRole('heading', { name: 'Achievement 13' }),
  ).not.toBeInTheDocument()
})
