import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { CardFace } from './CardFace'

afterEach(cleanup)

test('renders accessible card labels and state attributes', () => {
  render(<CardFace card={{ rank: '9', suit: 'Spades', playable: true, selected: true }} />)

  const card = screen.getByRole('button', { name: 'Play 9 of Spades' })
  expect(card).toHaveAttribute('data-playable', 'true')
  expect(card).toHaveAttribute('data-selected', 'true')
})
