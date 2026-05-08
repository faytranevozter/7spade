import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, expect, test } from 'vitest'
import App from './App'

afterEach(() => {
  cleanup()
})

function renderRoute(route: string) {
  return render(
    <MemoryRouter initialEntries={[route]}>
      <App />
    </MemoryRouter>,
  )
}

test('renders real top-level routes with temporary hardcoded data', () => {
  renderRoute('/auth')
  expect(screen.getByRole('heading', { name: /Auth entry/i })).toBeInTheDocument()
  cleanup()

  renderRoute('/lobby')
  expect(screen.getByRole('heading', { name: /Game lobby/i })).toBeInTheDocument()
  expect(screen.getByText(/Meja Santai #1/i)).toBeInTheDocument()
  cleanup()

  renderRoute('/results/room-1')
  expect(screen.getByRole('table', { name: /Score table/i })).toBeInTheDocument()
  cleanup()

  renderRoute('/history')
  expect(screen.getByRole('heading', { name: /Game history/i })).toBeInTheDocument()
  expect(screen.getByText(/XKQP7/i)).toBeInTheDocument()
})

test('renders a single dynamic game route', () => {
  renderRoute('/game/room-1')

  expect(screen.getByRole('heading', { name: /Live game table/i })).toBeInTheDocument()
  expect(screen.getByRole('region', { name: /Seven Spade game board/i })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Play card/i })).toBeInTheDocument()
})

test('temporary buttons navigate through the hardcoded flow', async () => {
  renderRoute('/lobby')

  fireEvent.click(screen.getAllByRole('button', { name: /Join/i })[0])
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Live game table/i })).toBeInTheDocument()
  })

  fireEvent.click(screen.getByRole('button', { name: /Play card/i }))
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Results and rematch/i })).toBeInTheDocument()
  })

  fireEvent.click(screen.getByRole('button', { name: /View history/i }))
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Game history/i })).toBeInTheDocument()
  })
})

test('redirects unknown routes to auth', async () => {
  renderRoute('/unknown')

  await waitFor(() => {
    expect(screen.getByRole('heading', { name: /Auth entry/i })).toBeInTheDocument()
  })
})

test('does not render prototype navigation', () => {
  renderRoute('/auth')

  expect(screen.getByText('Seven Spade')).toBeInTheDocument()
  expect(screen.queryByLabelText(/Prototype scenes/i)).not.toBeInTheDocument()
  expect(screen.queryByText(/Static React\/Tailwind prototype/i)).not.toBeInTheDocument()
})
