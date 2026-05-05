import '@testing-library/jest-dom/vitest'
import { render, screen, within } from '@testing-library/react'
import { expect, test } from 'vitest'
import App from './App'

test('renders the Seven Spade lobby, game board, and results states from the PRD', () => {
  render(<App />)

  expect(screen.getByRole('heading', { name: /Seven Spade/i })).toBeInTheDocument()
  expect(screen.getByRole('textbox', { name: /Display name/i })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Play as guest/i })).toBeInTheDocument()

  expect(screen.getByRole('heading', { name: /Open public rooms/i })).toBeInTheDocument()
  expect(screen.getByText(/Meja Santai #1/i)).toBeInTheDocument()

  const board = screen.getByRole('region', { name: /Seven Spade game board/i })
  expect(within(board).getAllByLabelText(/suit sequence/i)).toHaveLength(4)
  expect(screen.getByRole('button', { name: /Play 6 of Spades/i })).toHaveAttribute(
    'data-state',
    'playable',
  )
  expect(screen.getByRole('dialog', { name: /Choose a face-down penalty card/i })).toBeInTheDocument()

  expect(screen.getByRole('table', { name: /Final scoreboard/i })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Offer rematch/i })).toBeInTheDocument()
})
