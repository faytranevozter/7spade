import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { Badge } from './Badge'
import { SectionPanel } from './SectionPanel'

afterEach(cleanup)

test('renders heading, eyebrow, action, and children', () => {
  render(
    <SectionPanel title="Open rooms" eyebrow="Public" action={<Badge>3 waiting</Badge>}>
      <p>Room list</p>
    </SectionPanel>,
  )

  expect(screen.getByRole('heading', { name: 'Open rooms' })).toBeInTheDocument()
  expect(screen.getByText('Public')).toBeInTheDocument()
  expect(screen.getByText('3 waiting')).toBeInTheDocument()
  expect(screen.getByText('Room list')).toBeInTheDocument()
})
