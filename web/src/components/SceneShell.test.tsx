import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { Button } from './Button'
import { SceneShell } from './SceneShell'

afterEach(cleanup)

test('renders heading, eyebrow, action, and children', () => {
  render(
    <SceneShell title="Lobby" eyebrow="Rooms" action={<Button>New room</Button>}>
      <p>Room list</p>
    </SceneShell>,
  )

  expect(screen.getByRole('heading', { name: 'Lobby' })).toBeInTheDocument()
  expect(screen.getByText('Rooms')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'New room' })).toBeInTheDocument()
  expect(screen.getByText('Room list')).toBeInTheDocument()
})
