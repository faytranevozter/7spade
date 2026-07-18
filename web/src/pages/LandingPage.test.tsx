import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, expect, test } from 'vitest'
import { LandingPage } from './LandingPage'

afterEach(() => {
  cleanup()
})

test('landing page explains app purpose and links to legal pages', () => {
  render(
    <MemoryRouter>
      <LandingPage />
    </MemoryRouter>,
  )

  expect(screen.getByRole('heading', { name: /^Seven Spade$/i })).toBeInTheDocument()
  expect(screen.getByText(/purpose of the Seven Spade application/i)).toBeInTheDocument()
  expect(screen.getByText(/real-time multiplayer card game/i)).toBeInTheDocument()
  expect(screen.getByText(/lowest penalty/i)).toBeInTheDocument()
  expect(screen.getByRole('link', { name: /Privacy Policy/i })).toHaveAttribute('href', '/privacy')
  expect(screen.getByRole('link', { name: /Terms of Service/i })).toHaveAttribute('href', '/terms')
  expect(screen.getByRole('link', { name: /Sign in \/ Play/i })).toHaveAttribute('href', '/auth')
  expect(
    screen.getAllByRole('link', { name: /^Create account$/i }).every((el) => el.getAttribute('href') === '/register'),
  ).toBe(true)
})
