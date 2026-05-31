import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { Avatar } from './Avatar'

afterEach(() => {
  cleanup()
})

test('renders an image when an avatar URL is provided', () => {
  render(<Avatar avatarUrl="https://cdn/pic.png" initials="AL" alt="Alice" />)

  const img = screen.getByRole('img', { name: 'Alice' })
  expect(img).toHaveAttribute('src', 'https://cdn/pic.png')
  expect(img).toHaveAttribute('referrerpolicy', 'no-referrer')
})

test('renders initials when no avatar URL is provided', () => {
  render(<Avatar initials="AL" />)

  expect(screen.queryByRole('img')).not.toBeInTheDocument()
  expect(screen.getByText('AL')).toBeInTheDocument()
})

test('falls back to initials when the image fails to load', () => {
  render(<Avatar avatarUrl="https://cdn/broken.png" initials="AL" alt="Alice" />)

  const img = screen.getByRole('img', { name: 'Alice' })
  fireEvent.error(img)

  expect(screen.queryByRole('img')).not.toBeInTheDocument()
  expect(screen.getByText('AL')).toBeInTheDocument()
})

test('retries loading when the avatar URL changes after a prior failure', () => {
  const { rerender } = render(<Avatar avatarUrl="https://cdn/broken.png" initials="AL" alt="Alice" />)

  fireEvent.error(screen.getByRole('img', { name: 'Alice' }))
  expect(screen.queryByRole('img')).not.toBeInTheDocument()

  // A new URL on the same instance should attempt the image again.
  rerender(<Avatar avatarUrl="https://cdn/fresh.png" initials="AL" alt="Alice" />)
  const img = screen.getByRole('img', { name: 'Alice' })
  expect(img).toHaveAttribute('src', 'https://cdn/fresh.png')
})
