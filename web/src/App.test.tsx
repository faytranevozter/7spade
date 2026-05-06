import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, expect, test, vi } from 'vitest'
import App from './App'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function renderRoute(route: string) {
  return render(
    <MemoryRouter initialEntries={[route]}>
      <App />
    </MemoryRouter>,
  )
}

test('renders top-level static scene routes', () => {
  renderRoute('/mock/auth')
  expect(screen.getByRole('heading', { name: /Auth entry/i })).toBeInTheDocument()
  expect(screen.getByRole('textbox', { name: /Display name/i })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Continue to lobby/i })).toBeInTheDocument()
  cleanup()

  renderRoute('/mock/lobby')
  expect(screen.getByRole('heading', { name: /Game lobby/i })).toBeInTheDocument()
  expect(screen.getAllByText(/Meja Santai #1/i).length).toBeGreaterThan(0)
  expect(screen.queryByRole('dialog', { name: /Join private room/i })).not.toBeInTheDocument()
  cleanup()

  renderRoute('/mock/lobby/private-join')
  expect(screen.getByRole('dialog', { name: /Join private room/i })).toBeInTheDocument()
  cleanup()

  renderRoute('/mock/results')
  expect(screen.getByRole('table', { name: /Score table/i })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Vote rematch/i })).toBeInTheDocument()
  cleanup()

  renderRoute('/mock/results/rematch-ready')
  expect(screen.getByRole('heading', { name: /Rematch ready/i })).toBeInTheDocument()
  expect(screen.getByText(/4 \/ 4 voted/i)).toBeInTheDocument()
  cleanup()

  renderRoute('/mock/history')
  expect(screen.getByRole('heading', { name: /Game history/i })).toBeInTheDocument()
})

test('renders gameplay scenario routes', () => {
  renderRoute('/mock/game/my-turn/playable')
  const board = screen.getAllByRole('region', { name: /Seven Spade game board/i })[0]
  expect(within(board).getAllByLabelText(/suit sequence/i)).toHaveLength(4)
  expect(screen.getByRole('button', { name: /Play 6 of Spades/i })).toHaveAttribute(
    'data-playable',
    'true',
  )
  const selectedDiamonds = screen
    .getAllByRole('button', { name: /8 of Diamonds/i })
    .find((button) => button.getAttribute('data-selected') === 'true')

  expect(selectedDiamonds).toHaveAttribute(
    'data-selected',
    'true',
  )
  expect(screen.getByLabelText(/Face-down penalty pile/i)).toBeInTheDocument()
  cleanup()

  renderRoute('/mock/game/my-turn/no-valid-move')
  expect(screen.getByRole('heading', { name: /My turn - no valid move/i })).toBeInTheDocument()
  expect(screen.getByRole('dialog', { name: /Place face-down penalty/i })).toBeInTheDocument()
  cleanup()

  renderRoute('/mock/game/my-turn/invalid-move')
  expect(screen.getByRole('heading', { name: /My turn - invalid move/i })).toBeInTheDocument()
  cleanup()

  renderRoute('/mock/game/my-turn/timer-warning')
  expect(screen.getByRole('heading', { name: /My turn - timer warning/i })).toBeInTheDocument()
  cleanup()

  renderRoute('/mock/game/opponent-turn')
  expect(screen.getByRole('heading', { name: /Opponent turn/i })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Play card/i })).toBeDisabled()
  cleanup()

  renderRoute('/mock/game/opponent-played-card')
  expect(screen.getByText(/Budi played 9 Diamonds/i)).toBeInTheDocument()
  cleanup()

  renderRoute('/mock/game/opponent-passed')
  expect(screen.getByText(/Budi placed face-down/i)).toBeInTheDocument()
  cleanup()

  renderRoute('/mock/game/disconnected-player-bot')
  expect(screen.getByText(/Auto-play will handle their turns/i)).toBeInTheDocument()
  cleanup()

  renderRoute('/mock/game/reconnect')
  expect(screen.getByRole('button', { name: /Reconnect to room/i })).toBeInTheDocument()
  cleanup()

  renderRoute('/mock/game/round-ending')
  expect(screen.getByRole('heading', { name: /Round ending/i })).toBeInTheDocument()
})

test('removes standalone face-down selection scene', () => {
  renderRoute('/mock/game/my-turn/playable')

  expect(screen.queryByRole('link', { name: /Face-down/i })).not.toBeInTheDocument()
  expect(screen.queryByRole('heading', { name: /Face-down selection/i })).not.toBeInTheDocument()
})

test('redirects old clean paths away from mock scenes', () => {
  renderRoute('/game/my-turn/playable')

  expect(screen.getByRole('heading', { name: /Auth entry/i })).toBeInTheDocument()
  expect(screen.queryByRole('heading', { name: /My turn - playable card/i })).not.toBeInTheDocument()
})

test('renders static prototype status without backend health probes', () => {
  const fetchSpy = vi.fn()
  vi.stubGlobal('fetch', fetchSpy)

  renderRoute('/mock/game/my-turn/playable')

  expect(screen.getByText(/Static React\/Tailwind prototype/i)).toBeInTheDocument()
  expect(screen.getByText(/Card played/i)).toBeInTheDocument()
  expect(screen.queryByRole('heading', { name: /Service health/i })).not.toBeInTheDocument()
  expect(fetchSpy).not.toHaveBeenCalled()
})

test('renders private invite-code join controls in the static lobby', () => {
  renderRoute('/mock/lobby')

  expect(screen.getByRole('heading', { name: /Join private room/i })).toBeInTheDocument()
  expect(screen.getByRole('textbox', { name: /Invite code/i })).toHaveValue('XKQP7')
  expect(screen.getByRole('button', { name: /Join with code/i })).toBeInTheDocument()
})
