import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import App from './App'

afterEach(() => { cleanup(); window.location.hash = ''; vi.restoreAllMocks() })

test('achievement operator disables automatic grants and makes a reasoned exceptional grant', async () => {
  window.location.hash = '#/achievements'
  const admin = { id: '1', email: 'ops@example.com', display_name: 'Operator', status: 'active', permissions: ['achievements.read', 'achievements.manage', 'achievements.entitlements'] }
  const achievement = { id: 'first_win', name: 'First Blood', description: 'Win once', icon: 'trophy', display_order: 10, enabled: true, rules: [{ metric: 'is_winner', operator: 'eq', value: 'true' }], rules_locked: false }
  const fetchMock = vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => {
    const url = String(input)
    if (url.endsWith('/auth/refresh')) return new Response(JSON.stringify({ access_token: 'token', admin }), { status: 200 })
    if (url.endsWith('/sessions')) return new Response(JSON.stringify([]), { status: 200 })
    if (url.endsWith('/achievements') && (!init?.method || init.method === 'GET')) return new Response(JSON.stringify({ achievements: [achievement] }), { status: 200 })
    if (url.endsWith('/achievements/first_win') && init?.method === 'PUT') return new Response(JSON.stringify({ ...achievement, enabled: false }), { status: 200 })
    if (url.endsWith('/users/player-1/achievements/first_win/grant') && init?.method === 'POST') return new Response(JSON.stringify({ id: 'event-1' }), { status: 201 })
    throw new Error(`Unexpected request: ${url}`)
  })
  render(<App />)
  expect(await screen.findByRole('heading', { name: 'Achievements' })).toBeInTheDocument()
  fireEvent.click(await screen.findByRole('link', { name: /First Blood/ }))
  expect(await screen.findByRole('heading', { name: 'First Blood' })).toBeInTheDocument()
  expect(screen.getByLabelText('Metric')).toHaveValue('is_winner')
  expect(screen.getByLabelText('Operator')).toHaveValue('eq')
  expect(screen.getByLabelText('Value')).toHaveValue('true')
  fireEvent.click(screen.getByRole('button', { name: 'Add condition' }))
  expect(screen.getAllByLabelText('Metric')).toHaveLength(2)
  fireEvent.click(screen.getByRole('checkbox'))
  fireEvent.change(screen.getByLabelText('Change reason'), { target: { value: 'pause seasonal reward' } })
  fireEvent.click(screen.getByRole('button', { name: 'Save achievement' }))
  await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Achievement saved'))
  fireEvent.change(screen.getByLabelText('Player ID'), { target: { value: 'player-1' } })
  fireEvent.click(screen.getByRole('button', { name: 'Grant achievement' }))
  await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Achievement granted'))
  const grant = fetchMock.mock.calls.find(([input, init]) => String(input).endsWith('/users/player-1/achievements/first_win/grant') && init?.method === 'POST')
  expect(JSON.parse(String(grant?.[1]?.body))).toMatchObject({ reason: 'pause seasonal reward', idempotency_key: expect.any(String) })
})
