import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { GameBoard } from './GameBoard'
import type { BoardRow } from '../types'

afterEach(cleanup)

const boardRows: BoardRow[] = [
  { suit: 'Spades', cards: [null, null, null, null, null, null, '7', '8', null, null, null, null, null, null], stacks: { '7': 2 } },
  { suit: 'Hearts', cards: [null, null, null, null, null, null, '7', null, null, null, null, null, null, 'A'], closed: true },
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
