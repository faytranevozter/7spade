import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { BadgeGrid } from './BadgeGrid'

const catalog = [
  { id: 'first_win', name: 'First Blood', description: 'Win your first game', icon: '🏆' },
  { id: 'games_100', name: 'Centurion', description: 'Play 100 games', icon: '💯' },
]

afterEach(() => {
  cleanup()
})

test('shows only earned badges by default with an accurate count', () => {
  render(<BadgeGrid catalog={catalog} earned={['first_win']} />)

  // Only the earned badge is rendered by default.
  const items = screen.getAllByRole('listitem')
  expect(items).toHaveLength(1)
  expect(screen.getByText('First Blood')).toBeInTheDocument()
  expect(screen.queryByText('Centurion')).not.toBeInTheDocument()

  // Header reflects the earned count out of the full catalog.
  expect(screen.getByText(`1 / ${catalog.length} unlocked`)).toBeInTheDocument()
})

test('reveals the locked catalog via the Show all toggle', () => {
  render(<BadgeGrid catalog={catalog} earned={['first_win']} />)

  fireEvent.click(screen.getByRole('button', { name: /Show all \(1 locked\)/i }))

  expect(screen.getAllByRole('listitem')).toHaveLength(catalog.length)
  const list = screen.getByRole('list', { name: /Achievements/i })
  const firstWin = within(list).getByText('First Blood').closest('li')
  const locked = within(list).getByText('Centurion').closest('li')
  expect(firstWin?.className).not.toContain('opacity-50')
  expect(locked?.className).toContain('opacity-50')

  // Toggling back hides the locked badge again.
  fireEvent.click(screen.getByRole('button', { name: /Show earned only/i }))
  expect(screen.queryByText('Centurion')).not.toBeInTheDocument()
})

test('shows an empty hint when nothing is earned yet', () => {
  render(<BadgeGrid catalog={catalog} earned={[]} />)

  expect(screen.getByText(/No achievements unlocked yet/i)).toBeInTheDocument()
  expect(screen.getByText(`0 / ${catalog.length} unlocked`)).toBeInTheDocument()
})

test('shows an unavailable message when the API catalog is empty', () => {
  render(<BadgeGrid catalog={[]} earned={['first_win']} />)

  expect(screen.getByText('Achievement catalog unavailable.')).toBeInTheDocument()
  expect(screen.getByText('0 / 0 unlocked')).toBeInTheDocument()
})
