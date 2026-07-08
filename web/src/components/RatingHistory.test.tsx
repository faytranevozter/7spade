import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { RatingHistory } from './RatingHistory'

afterEach(cleanup)

test('renders nothing when empty and a sparkline when populated', () => {
  const { container, rerender } = render(<RatingHistory events={[]} />)
  expect(container).toBeEmptyDOMElement()

  rerender(
    <RatingHistory
      events={[
        { game_id: 'g2', rating_before: 1010, rating_after: 1005, rating_delta: -5, created_at: '2026-07-02T00:00:00Z' },
        { game_id: 'g1', rating_before: 1000, rating_after: 1010, rating_delta: 10, created_at: '2026-07-01T00:00:00Z' },
      ]}
    />,
  )

  expect(screen.getByRole('region', { name: 'Rating history' })).toBeInTheDocument()
  expect(screen.getByRole('img', { name: 'Rating from 1005 to 1010 over 2 games' })).toBeInTheDocument()
  expect(screen.getByText('now 1005')).toBeInTheDocument()
  expect(screen.getByText('-5')).toBeInTheDocument()
  expect(screen.getByText('+10')).toBeInTheDocument()
})
