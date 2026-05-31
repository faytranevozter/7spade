import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, within } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { BadgeGrid } from './BadgeGrid'
import { achievements } from '../game/achievements'

afterEach(() => {
  cleanup()
})

test('renders the full catalog and highlights earned badges', () => {
  render(<BadgeGrid earned={['first_win']} />)

  // The full catalog is rendered.
  const items = screen.getAllByRole('listitem')
  expect(items).toHaveLength(achievements.length)

  // Header reflects the earned count.
  expect(screen.getByText(`1 / ${achievements.length} unlocked`)).toBeInTheDocument()

  // Both an earned and a locked badge are present by name.
  expect(screen.getByText('First Blood')).toBeInTheDocument()
  expect(screen.getByText('Centurion')).toBeInTheDocument()
})

test('marks earned badges without the dimmed/locked opacity', () => {
  render(<BadgeGrid earned={['first_win']} />)

  const list = screen.getByRole('list', { name: /Achievements/i })
  const firstWin = within(list).getByText('First Blood').closest('li')
  const locked = within(list).getByText('Centurion').closest('li')

  expect(firstWin?.className).not.toContain('opacity-50')
  expect(locked?.className).toContain('opacity-50')
})
