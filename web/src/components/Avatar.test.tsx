import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { Avatar } from './Avatar'

const useSkinAssetState = vi.hoisted(() => vi.fn())

vi.mock('../hooks/useSkinAsset', () => ({
  useSkinAsset: vi.fn(() => null),
  useSkinAssetState,
}))

useSkinAssetState.mockReturnValue({ url: null, isLoading: false })

afterEach(() => {
  cleanup()
  useSkinAssetState.mockReset()
  useSkinAssetState.mockReturnValue({ url: null, isLoading: false })
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

test('keeps the fallback avatar hidden while equipped skin metadata loads', () => {
  render(<Avatar avatarUrl="https://cdn/profile.png" initials="AL" alt="Alice" displayPictureLoading />)

  expect(screen.getByRole('status', { name: 'Loading profile picture' })).toBeInTheDocument()
  expect(screen.queryByRole('img', { name: 'Alice' })).not.toBeInTheDocument()
})

test('keeps the fallback avatar hidden while a display-picture asset resolves', () => {
  useSkinAssetState.mockReturnValue({ url: null, isLoading: true })
  render(
    <Avatar
      avatarUrl="https://cdn/profile.png"
      initials="AL"
      alt="Alice"
      displayPictureSkinId="display-picture"
      displayPictureAssetKey="display-pictures/alice.png"
    />,
  )

  expect(screen.getByRole('status', { name: 'Loading profile picture' })).toBeInTheDocument()
  expect(screen.queryByRole('img', { name: 'Alice' })).not.toBeInTheDocument()
})

test('uses the resolved display picture instead of the profile avatar', () => {
  useSkinAssetState.mockReturnValue({ url: 'https://skins/display.png', isLoading: false })
  render(
    <Avatar
      avatarUrl="https://cdn/profile.png"
      initials="AL"
      alt="Alice"
      displayPictureSkinId="display-picture"
      displayPictureAssetKey="display-pictures/alice.png"
    />,
  )

  expect(screen.getByRole('img', { name: 'Alice' })).toHaveAttribute('src', 'https://skins/display.png')
})

test('falls back to the profile avatar when a display-picture asset cannot load', () => {
  useSkinAssetState.mockReturnValue({ url: null, isLoading: false })
  render(
    <Avatar
      avatarUrl="https://cdn/profile.png"
      initials="AL"
      alt="Alice"
      displayPictureSkinId="display-picture"
      displayPictureAssetKey="display-pictures/alice.png"
    />,
  )

  expect(screen.getByRole('img', { name: 'Alice' })).toHaveAttribute('src', 'https://cdn/profile.png')
})
