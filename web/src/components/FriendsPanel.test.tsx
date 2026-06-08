import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { FriendsPanel } from './FriendsPanel'
import { acceptFriendRequest, getFriends, removeFriend, searchUsers, sendFriendRequest } from '../api/friends'

vi.mock('../api/friends', () => ({
  getFriends: vi.fn(),
  sendFriendRequest: vi.fn(),
  acceptFriendRequest: vi.fn(),
  removeFriend: vi.fn(),
  searchUsers: vi.fn(),
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
  vi.mocked(searchUsers).mockResolvedValue({
    results: [{ user_id: 'u9', username: 'dave', display_name: 'Dave', avatar_url: null }],
  })
  vi.mocked(sendFriendRequest).mockResolvedValue({ status: 'pending' })
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

test('the add-friend modal searches and sends a request by user id', async () => {
  renderPanel()
  await waitFor(() => expect(screen.getByText('Alice')).toBeInTheDocument())

  fireEvent.click(screen.getByRole('button', { name: /Add friend/i }))

  const input = await screen.findByRole('textbox', { name: /Search players/i })
  fireEvent.change(input, { target: { value: 'dave' } })

  // Debounced search (300ms) fires and renders the result row.
  await waitFor(() => {
    expect(searchUsers).toHaveBeenCalledWith('test-token', 'dave')
  })
  await waitFor(() => expect(screen.getByText('Dave')).toBeInTheDocument())

  // Each result row carries its own "Add friend" action (send by user_id).
  const addButtons = screen.getAllByRole('button', { name: /Add friend/i })
  fireEvent.click(addButtons[addButtons.length - 1])

  await waitFor(() => {
    expect(sendFriendRequest).toHaveBeenCalledWith('test-token', { userId: 'u9' })
  })
})
