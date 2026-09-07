import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
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

function renderSettings(canWrite = true) {
  vi.mocked(getApplicationSettings).mockResolvedValue([
    { key: 'room_creation', enabled: true },
    { key: 'quick_play', enabled: true },
  ])
  return render(<AuthContext.Provider value={{
    token: 'token', admin: { id: '1', email: 'ops@example.com', display_name: 'Ops', status: 'active', permissions: canWrite ? ['settings.read', 'settings.write'] : ['settings.read'] },
    challengeToken: '', isLoading: false, error: '', signIn: vi.fn(), completeMFA: vi.fn(), signOut: vi.fn(), refreshSession: vi.fn(), expireSession: vi.fn(),
  }}><SettingsPage /></AuthContext.Provider>)
}

test('cards save independently and retain their own errors, drafts and status', async () => {
  let rejectRoom!: (error: Error) => void
  let resolveQuick!: (value: { key: string; enabled: boolean }) => void
  vi.mocked(updateApplicationSetting).mockImplementation((_token, key) => new Promise((resolve, reject) => {
    if (key === 'room_creation') rejectRoom = reject
    else resolveQuick = resolve
  }))
  renderSettings()
  const room = within(screen.getByRole('form', { name: 'Room creation' }))
  const quick = within(screen.getByRole('form', { name: 'Quick Play' }))
  await waitFor(() => expect(room.getByRole('checkbox')).toBeEnabled())
  for (const card of [room, quick]) {
    fireEvent.click(card.getByRole('checkbox'))
    fireEvent.change(card.getByRole('textbox'), { target: { value: 'Maintenance' } })
    expect(card.getByText(/Unsaved changes/)).toBeInTheDocument()
    expect(card.getByText(/Currently enabled/)).toBeInTheDocument()
    fireEvent.click(card.getByRole('button'))
  }
  expect(updateApplicationSetting).toHaveBeenCalledTimes(2)
  expect(room.getByRole('textbox')).toBeDisabled()
  await act(async () => rejectRoom(new Error('Room save failed')))
  expect(room.getByText('Room save failed')).toBeInTheDocument()
  expect(room.getByRole('textbox')).toHaveValue('Maintenance')
  expect(quick.getByRole('button')).toHaveTextContent('Saving...')
  await act(async () => resolveQuick({ key: 'quick_play', enabled: false }))
  expect(quick.getByRole('status')).toHaveTextContent('Quick Play disabled')
  expect(quick.getByText('Currently disabled')).toBeInTheDocument()
  expect(quick.queryByText(/Unsaved changes/)).not.toBeInTheDocument()
  expect(room.getByText('Room save failed')).toBeInTheDocument()
  expect(room.getByText(/Unsaved changes/)).toBeInTheDocument()
  expect(room.getByRole('button')).toBeEnabled()
})

test('reason-only and reverted drafts cannot submit no-op saves', async () => {
  renderSettings()
  const form = screen.getByRole('form', { name: 'Room creation' })
  const card = within(form)
  await waitFor(() => expect(card.getByRole('checkbox')).toBeEnabled())
  fireEvent.change(card.getByRole('textbox'), { target: { value: 'Reason only' } })
  expect(card.getByRole('button')).toBeDisabled()
  fireEvent.submit(form)
  fireEvent.click(card.getByRole('checkbox'))
  expect(card.getByRole('button')).toBeEnabled()
  fireEvent.click(card.getByRole('checkbox'))
  expect(card.getByRole('button')).toBeDisabled()
  expect(card.queryByText(/Unsaved changes/)).not.toBeInTheDocument()
  fireEvent.submit(form)
  expect(updateApplicationSetting).not.toHaveBeenCalled()
})

test('read-only administrators see persisted values without editing or saving', async () => {
  renderSettings(false)
  const form = screen.getByRole('form', { name: 'Room creation' })
  const card = within(form)
  await waitFor(() => expect(card.getByRole('checkbox')).toBeChecked())
  expect(card.getByRole('checkbox')).toBeDisabled()
  expect(card.getByText('Currently enabled')).toBeInTheDocument()
  expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  expect(screen.queryByRole('button')).not.toBeInTheDocument()
  fireEvent.submit(form)
  expect(updateApplicationSetting).not.toHaveBeenCalled()
})
