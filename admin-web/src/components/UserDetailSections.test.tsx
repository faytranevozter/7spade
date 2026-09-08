import '@testing-library/jest-dom/vitest'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { MemoryRouter, useLocation, useNavigate } from 'react-router'
import type { UserDetail } from '../api/users'
import { UserAccountPanel, UserDetailSections } from './UserDetailSections'

afterEach(cleanup)
const detail: UserDetail = {
  user: {
    id: 'user-1',
    username: 'ace',
    display_name: 'Ace',
    version: 3,
    created_at: '2026-08-01T00:00:00Z',
    online: false,
  },
  stats: {},
  providers: ['google'],
  skins: Array.from({ length: 13 }, (_, index) => ({
    id: `skin-${index}`,
    name: `Skin ${index}`,
    skin_type: index === 12 ? 'profile_background' : 'avatar_frame',
    source: index === 12 ? 'admin_grant' : 'achievement',
    revision_id: 'historical-revision',
    asset_url: `https://cdn.example/old-${index}.png`,
    earned_at: '2026-08-01T00:00:00Z',
  })),
  achievements: [
    {
      achievement_id: 'first-win',
      name: 'First win',
      description: 'Win a game',
      icon: 'W',
      earned_at: '2026-08-01T00:00:00Z',
    },
  ],
  games: [
    {
      id: 'game-1',
      room_id: 'room-1',
      rank: 1,
      penalty_points: 0,
      finished_at: null,
    },
  ],
  ratings: [
    {
      rating_before: 1000,
      rating_after: 1010,
      rating_delta: 10,
      created_at: '2026-08-01T00:00:00Z',
    },
  ],
  room: { id: 'room-1', status: 'waiting', created_at: '2026-08-01T00:00:00Z' },
}
function Location() {
  const location = useLocation()
  const navigate = useNavigate()
  return (
    <>
      <output aria-label="Location">{location.search}</output>
      <button onClick={() => navigate(-1)}>Back</button>
    </>
  )
}
function show(section: string, permissions: string[] = [], value = detail) {
  return render(
    <MemoryRouter
      initialEntries={[`/users/user-1?section=${section}&keep=yes`]}
    >
      <UserDetailSections detail={value} permissions={permissions} />
      <UserAccountPanel detail={value} permissions={permissions} />
      <Location />
    </MemoryRouter>,
  )
}

test('section query survives navigation and back; unknown sections show overview without fake zeros', async () => {
  const { container } = show('unknown')
  expect(
    screen.getByRole('heading', { name: 'Progression snapshot' }),
  ).toBeInTheDocument()
  expect(screen.getAllByText('Not available')).toHaveLength(5)
  expect(container.querySelector('main')).toBeNull()
  fireEvent.click(screen.getByRole('link', { name: 'Achievements' }))
  expect(screen.getByLabelText('Location')).toHaveTextContent(
    'section=achievements&keep=yes',
  )
  expect(screen.getByText('Win a game')).toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: /grant|revoke/i }),
  ).not.toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Back' }))
  expect(
    await screen.findByRole('heading', { name: 'Progression snapshot' }),
  ).toBeInTheDocument()
})

test('owned skins paginate, filter by source/type, search, and recover from broken previews', () => {
  show('skins')
  expect(screen.getAllByRole('article')).toHaveLength(12)
  fireEvent.error(screen.getByAltText('Skin 0 owned preview'))
  expect(screen.getByText('Preview unavailable')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Next' }))
  expect(screen.getAllByRole('article')).toHaveLength(1)
  expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
  fireEvent.change(screen.getByLabelText('Source'), {
    target: { value: 'achievement' },
  })
  expect(screen.getByText('Page 1 of 1')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
  fireEvent.change(screen.getByLabelText('Skin type'), {
    target: { value: 'profile_background' },
  })
  expect(screen.getByText(/No matching rewards/)).toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Source'), { target: { value: '' } })
  expect(screen.getByAltText('Skin 12 owned preview')).toHaveClass(
    'aspect-admin-profile',
  )
  fireEvent.change(screen.getByLabelText('Search skins'), {
    target: { value: 'SKIN-12' },
  })
  expect(screen.getByText('1 of 13 skins')).toHaveAttribute('role', 'status')
  expect(
    screen.getByText('Owned revision: historical-revision'),
  ).toBeInTheDocument()
})

test('achievement search includes descriptions and empty collections are explicit', () => {
  show('achievements')
  fireEvent.change(screen.getByLabelText('Search achievements'), {
    target: { value: 'win a game' },
  })
  expect(screen.getByRole('article')).toHaveTextContent('First win')
  cleanup()
  show('achievements', [], { ...detail, achievements: [] })
  expect(
    screen.getByRole('heading', { name: 'No achievements earned' }),
  ).toBeInTheDocument()
  expect(
    screen.getByText(/In-progress achievements are not reported/),
  ).toBeInTheDocument()
})

test('empty activity uses descriptive headings without announcing an error', () => {
  show('activity', [], { ...detail, games: [], ratings: [] })
  expect(
    screen.getAllByRole('heading', { name: 'No records retained', level: 3 }),
  ).toHaveLength(2)
  expect(
    screen.getByText(/No game history records are available/),
  ).toBeInTheDocument()
  expect(
    screen.getByText(/No rating history records are available/),
  ).toBeInTheDocument()
  expect(screen.queryByRole('table')).not.toBeInTheDocument()
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  for (const button of screen.getAllByRole('button', {
    name: /Previous|Next/,
  })) {
    expect(button).toBeDisabled()
  }
})

test('empty ownership differs from filtered results and clearing search restores rewards', () => {
  show('skins', [], { ...detail, skins: [] })
  expect(
    screen.getByRole('heading', { name: 'No skins owned' }),
  ).toBeInTheDocument()
  expect(
    screen.getByText('This user has no skin ownership records to display.'),
  ).toBeInTheDocument()
  cleanup()
  show('achievements')
  fireEvent.change(screen.getByLabelText('Search achievements'), {
    target: { value: 'no-such-reward' },
  })
  expect(
    screen.getByRole('heading', { name: 'No matching rewards' }),
  ).toBeInTheDocument()
  expect(
    screen.getByText(/Adjust them to see owned rewards/),
  ).toBeInTheDocument()
  expect(
    screen.queryByRole('heading', { name: 'No achievements earned' }),
  ).not.toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Search achievements'), {
    target: { value: '' },
  })
  expect(screen.getByRole('article')).toHaveTextContent('First win')
})

test('activity and room links require their own permissions and show bounded history', () => {
  show('activity')
  expect(screen.getAllByText(/Up to the latest 100 records/)).toHaveLength(2)
  expect(screen.queryByRole('link', { name: 'game-1' })).not.toBeInTheDocument()
  expect(screen.queryByRole('link', { name: 'room-1' })).not.toBeInTheDocument()
  expect(
    within(screen.getAllByRole('table')[0]).getByText('Not available'),
  ).toBeInTheDocument()
  expect(screen.getByText('Restricted')).toBeInTheDocument()
  cleanup()
  show('activity', ['games.read', 'rooms.read', 'users.sensitive.read'])
  expect(screen.getByRole('link', { name: 'game-1' })).toHaveAttribute(
    'href',
    '/games/game-1',
  )
  expect(screen.getAllByRole('link', { name: 'room-1' })).toHaveLength(2)
  expect(screen.getByText('Not recorded')).toBeInTheDocument()
})
