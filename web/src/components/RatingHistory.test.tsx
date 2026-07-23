import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { RatingHistory } from './RatingHistory'

afterEach(cleanup)

const sampleEvents = [
  {
    game_id: 'g2',
    rating_before: 1010,
    rating_after: 1005,
    rating_delta: -5,
    created_at: '2026-07-02T00:00:00Z',
  },
  {
    game_id: 'g1',
    rating_before: 1000,
    rating_after: 1010,
    rating_delta: 10,
    created_at: '2026-07-01T00:00:00Z',
  },
]

test('renders nothing when empty', () => {
  const { container } = render(<RatingHistory events={[]} />)
  expect(container).toBeEmptyDOMElement()
})

test('renders chart summary and recent games when populated', () => {
  render(<RatingHistory events={sampleEvents} />)

  expect(screen.getByRole('region', { name: 'Rating history' })).toBeInTheDocument()
  expect(screen.getByRole('img', { name: 'Rating from 1005 to 1010 over 2 games' })).toBeInTheDocument()
  expect(screen.getByText('Now')).toBeInTheDocument()
  expect(screen.getByText('Net')).toBeInTheDocument()
  expect(screen.getByText('+5')).toBeInTheDocument()
  expect(screen.getByText('Peak')).toBeInTheDocument()
  expect(screen.getByText('Low')).toBeInTheDocument()
  expect(screen.getByText('-5')).toBeInTheDocument()
  expect(screen.getByText('+10')).toBeInTheDocument()
  expect(screen.getByText('Recent games')).toBeInTheDocument()
})

test('shows tooltip details on point focus', () => {
  render(<RatingHistory events={sampleEvents} />)

  fireEvent.focus(screen.getByRole('button', { name: 'Game rating 1010, +10' }))

  const status = screen.getByRole('status')
  expect(status).toHaveTextContent('1010')
  expect(status).toHaveTextContent('(+10)')
})
