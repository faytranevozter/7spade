import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { ProviderBadge } from './ProviderBadge'

afterEach(cleanup)

test('renders branded badge with icon for known providers', () => {
  render(<ProviderBadge provider="google" />)
  expect(screen.getByText('Google')).toBeInTheDocument()
  expect(document.querySelector('svg')).toBeInTheDocument()
})

test('falls back to raw name for unknown providers', () => {
  render(<ProviderBadge provider="discord" />)
  expect(screen.getByText('discord')).toBeInTheDocument()
})
