import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { EmoteBubble } from './EmoteBubble'

afterEach(cleanup)

test('renders valid emotes and ignores missing emotes', () => {
  const { rerender } = render(<EmoteBubble emote={{ id: 'gg', seq: 1 }} />)

  expect(screen.getByRole('status', { name: 'Emote: GG' })).toBeInTheDocument()

  rerender(<EmoteBubble emote={{ id: 'missing', seq: 2 }} />)
  expect(screen.queryByRole('status')).not.toBeInTheDocument()
})
