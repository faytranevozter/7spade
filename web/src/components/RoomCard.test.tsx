import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { RoomCard } from './RoomCard'
import type { Room } from '../types'

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

test('renders room details and disables full rooms', () => {
  const room: Room = {
    name: 'Ranked table',
    code: 'ABC123',
    players: '4 players',
    status: 'Waiting',
    timer: '60s',
    botDifficulty: 'medium',
    eloRange: '1000-1400 ELO',
    open: false,
    filledSeats: 4,
    maxSeats: 4,
    visibility: 'public',
  }
  const onJoin = vi.fn()

  render(<RoomCard room={room} onJoin={onJoin} />)
  fireEvent.click(screen.getByRole('button', { name: 'Full' }))

  expect(screen.getByText('Ranked table')).toBeInTheDocument()
  expect(screen.getByText('ABC123')).toBeInTheDocument()
  expect(screen.getByText('1000-1400 ELO')).toBeInTheDocument()
  expect(screen.getByLabelText('4 of 4 seats filled')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Full' })).toBeDisabled()
  expect(onJoin).not.toHaveBeenCalled()
})
