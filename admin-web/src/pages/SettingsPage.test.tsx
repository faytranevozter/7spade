import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { getDailyLoginSetting, updateDailyLoginSetting } from '../api/settings'
import { AuthContext } from '../hooks/useAuth'
import { SettingsPage } from './SettingsPage'

vi.mock('../api/settings', () => ({
  getDailyLoginSetting: vi.fn(),
  updateDailyLoginSetting: vi.fn(),
}))

afterEach(() => {
  cleanup()
  vi.resetAllMocks()
})

test('administrator disables daily login with an audit reason', async () => {
  vi.mocked(getDailyLoginSetting).mockResolvedValue({ key: 'daily_login', enabled: true })
  vi.mocked(updateDailyLoginSetting).mockResolvedValue({ key: 'daily_login', enabled: false })
  render(
    <AuthContext.Provider value={{ token: 'token', admin: { id: '1', email: 'ops@example.com', display_name: 'Ops', status: 'active', permissions: ['settings.read', 'settings.write'] }, challengeToken: '', isLoading: false, error: '', signIn: vi.fn(), completeMFA: vi.fn(), signOut: vi.fn(), refreshSession: vi.fn(), expireSession: vi.fn() }}>
      <SettingsPage />
    </AuthContext.Provider>,
  )

  expect(await screen.findByRole('checkbox', { name: 'Daily login enabled' })).toBeChecked()
  fireEvent.click(screen.getByRole('checkbox', { name: 'Daily login enabled' }))
  fireEvent.change(screen.getByLabelText('Reason for change'), { target: { value: 'Pause rewards during maintenance' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save setting' }))

  await waitFor(() => expect(updateDailyLoginSetting).toHaveBeenCalledWith('token', false, 'Pause rewards during maintenance'))
  expect(await screen.findByRole('status')).toHaveTextContent('Daily login disabled')
})
