import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, within } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import App from './App'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

test('renders static Seven Spade pages from the PRD', () => {
  render(<App />)

  expect(screen.getByRole('heading', { name: /Frontend design foundation/i })).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: /Auth entry/i })).toBeInTheDocument()
  expect(screen.getByRole('textbox', { name: /Display name/i })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Continue to lobby/i })).toBeInTheDocument()

  expect(screen.getByRole('heading', { name: /Game lobby/i })).toBeInTheDocument()
  expect(screen.getAllByText(/Meja Santai #1/i).length).toBeGreaterThan(0)

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
  expect(screen.getByRole('dialog', { name: /Choose one penalty card/i })).toBeInTheDocument()

  expect(screen.getByRole('table', { name: /Score table/i })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Vote rematch/i })).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: /Game history/i })).toBeInTheDocument()
})

test('renders static prototype status without backend health probes', () => {
  const fetchSpy = vi.fn()
  vi.stubGlobal('fetch', fetchSpy)

  render(<App />)

  expect(screen.getByText(/Static React\/Tailwind prototype/i)).toBeInTheDocument()
  expect(screen.getByText(/No backend calls/i)).toBeInTheDocument()
  expect(screen.getByText(/Card played/i)).toBeInTheDocument()
  expect(screen.queryByRole('heading', { name: /Service health/i })).not.toBeInTheDocument()
  expect(fetchSpy).not.toHaveBeenCalled()
})
