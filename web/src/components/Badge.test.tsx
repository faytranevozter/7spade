import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { Badge } from './Badge'

afterEach(cleanup)

test('renders its label', () => {
  render(<Badge tone="playing">Open</Badge>)

  expect(screen.getByText('Open')).toBeInTheDocument()
})
