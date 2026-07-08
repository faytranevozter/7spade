import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { EmotePicker } from './EmotePicker'

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

test('opens the tray, selects an emote, and respects disabled state', () => {
  const onSelect = vi.fn()
  const { rerender } = render(<EmotePicker onSelect={onSelect} />)

  fireEvent.click(screen.getByRole('button', { name: 'Open emotes' }))
  expect(screen.getByRole('menu', { name: 'Emotes' })).toBeInTheDocument()

  fireEvent.click(screen.getByRole('menuitem', { name: 'GG' }))
  expect(onSelect).toHaveBeenCalledWith('gg')
  expect(screen.queryByRole('menu', { name: 'Emotes' })).not.toBeInTheDocument()

  rerender(<EmotePicker onSelect={onSelect} disabled />)
  expect(screen.getByRole('button', { name: 'Open emotes' })).toBeDisabled()
})
