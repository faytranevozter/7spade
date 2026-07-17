import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { GameBoard } from './GameBoard'
import type { BoardRow } from '../types'

afterEach(cleanup)

const boardRows: BoardRow[] = [
  { suit: 'Spades', cards: [null, null, null, null, null, null, '7', '8', null, null, null, null, null, null], stacks: { '7': 2 } },
  { suit: 'Hearts', cards: [null, null, null, null, null, null, '7', null, null, null, null, null, null, 'A'], closed: true },
  { suit: 'Diamonds', cards: [null, null, null, null, null, null, '7', null, null, null, null, null, null, null] },
  { suit: 'Clubs', cards: [null, null, null, null, null, null, '7', null, null, null, null, null, null, null] },
]

test('renders board rows, closed suits, and stack counts', () => {
  render(<GameBoard rows={boardRows} />)

  expect(screen.getByRole('region', { name: 'Seven Spade game board' })).toBeInTheDocument()
  expect(screen.getByLabelText('Spades suit sequence')).toBeInTheDocument()
  expect(screen.getByLabelText('Hearts suit sequence, closed')).toHaveTextContent('Closed')
  expect(screen.getByRole('button', { name: '7 of Spades' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'A of Hearts' })).toBeInTheDocument()
  expect(screen.getByText('2')).toBeInTheDocument()
})

test('fits a narrow phone-width container without horizontal overflow', () => {
  // Simulate a phone-width parent (iPhone SE / small Android).
  const host = document.createElement('div')
  host.style.width = '320px'
  host.style.overflow = 'hidden'
  document.body.appendChild(host)

  render(<GameBoard rows={boardRows} />, { container: host })

  const board = screen.getByRole('region', { name: 'Seven Spade game board' })
  expect(board.className).not.toMatch(/overflow-x-auto/)
  expect(board.className).toMatch(/overflow-hidden/)

  // Content must not force a wider scroll width than the phone container.
  expect(board.scrollWidth).toBeLessThanOrEqual(board.clientWidth + 1)
  expect(host.scrollWidth).toBeLessThanOrEqual(host.clientWidth + 1)

  host.remove()
})
