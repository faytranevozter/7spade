import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { FriendsPanel } from './FriendsPanel'
import { acceptFriendRequest, getFriends, removeFriend } from '../api/friends'

vi.mock('../api/friends', () => ({
  getFriends: vi.fn(),
  sendFriendRequest: vi.fn(),
  acceptFriendRequest: vi.fn(),
  removeFriend: vi.fn(),
}))

beforeEach(() => {
  vi.mocked(getFriends).mockResolvedValue({
    friends: [
      { user_id: 'u1', display_name: 'Alice', username: 'alice', avatar_url: null, status: 'accepted', online: true, room_id: 'room-9' },
      { user_id: 'u2', display_name: 'Bob', username: 'bob', avatar_url: null, status: 'accepted', online: false },
      { user_id: 'u3', display_name: 'Carol', username: 'carol', avatar_url: null, status: 'incoming', online: false },
    ],
  })
  vi.mocked(acceptFriendRequest).mockResolvedValue(undefined)
  vi.mocked(removeFriend).mockResolvedValue(undefined)
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

function renderPanel() {
  return render(
    <MemoryRouter initialEntries={['/lobby']}>
      <Routes>
        <Route path="/lobby" element={<FriendsPanel token="test-token" refreshNonce={0} />} />
        <Route path="/watch/:roomId" element={<div>Watching room</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

test('renders accepted friends with presence and incoming requests', async () => {
  renderPanel()

  await waitFor(() => {
    expect(screen.getByText('Alice')).toBeInTheDocument()
  })
  expect(screen.getByText('In a game')).toBeInTheDocument()
  expect(screen.getByText('Offline')).toBeInTheDocument()
  // Incoming request shows an Accept button.
  expect(screen.getByText('Carol')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Accept/i })).toBeInTheDocument()
})

test('accepting a request calls the API', async () => {
  renderPanel()
  await waitFor(() => expect(screen.getByText('Carol')).toBeInTheDocument())

  fireEvent.click(screen.getByRole('button', { name: /Accept/i }))

  await waitFor(() => {
    expect(acceptFriendRequest).toHaveBeenCalledWith('test-token', 'u3')
  })
})

test('an in-game friend offers a Watch link', async () => {
  renderPanel()
  await waitFor(() => expect(screen.getByText('Alice')).toBeInTheDocument())

  fireEvent.click(screen.getByRole('button', { name: /Watch/i }))

  await waitFor(() => {
    expect(screen.getByText('Watching room')).toBeInTheDocument()
  })
})
