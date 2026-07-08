import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { ScoreTable } from './ScoreTable'
import type { Score } from '../types'

afterEach(cleanup)

test('renders scores and custom winner label', () => {
  const scores: Score[] = [
    { rank: 1, player: 'Alice', cardsLeft: 0, penalty: 0, result: 'Winner', winner: true },
    { rank: 2, player: 'Bob', cardsLeft: 5, penalty: 20, result: 'Finished', me: true },
  ]

  render(<ScoreTable scores={scores} winnerLabel="Champion" />)

  expect(screen.getByRole('table', { name: 'Score table' })).toBeInTheDocument()
  expect(screen.getByText('Alice')).toBeInTheDocument()
  expect(screen.getByText('Champion')).toBeInTheDocument()
  expect(screen.getByText('Bob')).toBeInTheDocument()
  expect(screen.getByText('20')).toBeInTheDocument()
})
