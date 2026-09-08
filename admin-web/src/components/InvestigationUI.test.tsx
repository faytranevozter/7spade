import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { EmptyState } from './InvestigationUI'

afterEach(cleanup)

test('compact empty state opts out of the default oversized panel', () => {
  const { container } = render(
    <EmptyState compact mark="0" title="No skins" description="No skins owned." />,
  )
  expect(container.firstChild).toHaveClass('admin-empty-compact')
  expect(container.firstChild).not.toHaveClass('min-h-75')
  expect(screen.getByRole('heading', { name: 'No skins', level: 3 })).toBeInTheDocument()
  expect(screen.getByText('0')).toHaveAttribute('aria-hidden', 'true')
})

test('empty state presents a heading and guidance with a decorative mark, not an alert', () => {
  const { container } = render(
    <EmptyState
      mark="0"
      title="No records retained"
      description="No records are available for this user."
      className="mt-6"
      markClassName="text-xl"
    />,
  )
  expect(
    screen.getByRole('heading', { name: 'No records retained', level: 3 }),
  ).toBeInTheDocument()
  expect(
    screen.getByText('No records are available for this user.'),
  ).toBeInTheDocument()
  expect(screen.getByText('0')).toHaveAttribute('aria-hidden', 'true')
  expect(screen.getByText('0')).toHaveClass('text-xl')
  expect(container.firstChild).toHaveClass(
    'mt-6',
    'border',
    'bg-admin-accent-soft',
  )
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  expect(screen.queryByRole('status')).not.toBeInTheDocument()
})
