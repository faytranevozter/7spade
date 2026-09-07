import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { getApplicationSettings, updateApplicationSetting } from '../api/settings'
import { AuthContext } from '../hooks/useAuth'
import { SettingsPage } from './SettingsPage'

vi.mock('../api/settings', () => ({
  getApplicationSettings: vi.fn(),
  updateApplicationSetting: vi.fn(),
}))

afterEach(() => {
  cleanup()
  vi.resetAllMocks()
})

test('administrator disables room creation with an audit reason', async () => {
  vi.mocked(getApplicationSettings).mockResolvedValue([
    { key: 'daily_login', enabled: true },
    { key: 'new_registrations', enabled: true },
    { key: 'guest_access', enabled: true },
    { key: 'room_creation', enabled: true },
    { key: 'quick_play', enabled: true },
  ])
  vi.mocked(updateApplicationSetting).mockResolvedValue({ key: 'room_creation', enabled: false })
  render(
    <AuthContext.Provider value={{
      token: 'token', admin: { id: '1', email: 'ops@example.com', display_name: 'Ops', status: 'active', permissions: ['settings.read', 'settings.write'] },
      challengeToken: '', isLoading: false, error: '', signIn: vi.fn(), completeMFA: vi.fn(), signOut: vi.fn(), refreshSession: vi.fn(), expireSession: vi.fn(),
    }}>
      <SettingsPage />
    </AuthContext.Provider>,
  )

  const toggle = await screen.findByRole('checkbox', { name: 'Room creation enabled' })
  expect(toggle).toBeChecked()
  fireEvent.click(toggle)
  fireEvent.change(screen.getByLabelText('Room creation reason for change'), { target: { value: 'Maintenance window' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save Room creation' }))

  await waitFor(() => expect(updateApplicationSetting).toHaveBeenCalledWith('token', 'room_creation', false, 'Maintenance window'))
  expect(await screen.findByRole('status')).toHaveTextContent('Room creation disabled')
})
