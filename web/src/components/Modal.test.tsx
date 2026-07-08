import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { Button } from './Button'
import { Modal } from './Modal'

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

test('renders dialog content and closes from Escape and close button', () => {
  const onClose = vi.fn()

  render(
    <Modal title="Create room" eyebrow="Lobby" description="Pick room settings." onClose={onClose} footer={<Button>Save</Button>}>
      <p>Room form</p>
    </Modal>,
  )

  const dialog = screen.getByRole('dialog', { name: 'Create room' })
  expect(dialog).toHaveTextContent('Lobby')
  expect(dialog).toHaveTextContent('Pick room settings.')
  expect(dialog).toHaveTextContent('Room form')
  expect(within(dialog).getByRole('button', { name: 'Save' })).toBeInTheDocument()

  fireEvent.keyDown(document, { key: 'Escape' })
  fireEvent.click(within(dialog).getByRole('button', { name: 'Close Create room' }))

  expect(onClose).toHaveBeenCalledTimes(2)
})
