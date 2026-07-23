import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router'
import { ProfileTabs } from './ProfileTabs'

function LocationProbe() {
  const { search } = useLocation()
  return <div data-testid="search">{search}</div>
}

function renderTabs(initial = '/players/u1') {
  return render(
    <MemoryRouter initialEntries={[initial]}>
      <Routes>
        <Route
          path="/players/:id"
          element={
            <>
              <ProfileTabs
                tabs={[
                  { id: 'overview', label: 'Overview', panel: <div>Overview panel</div> },
                  { id: 'rating', label: 'Rating', panel: <div>Rating panel</div> },
                  { id: 'achievements', label: 'Achievements', panel: <div>Achievements panel</div> },
                ]}
              />
              <LocationProbe />
            </>
          }
        />
      </Routes>
    </MemoryRouter>,
  )
}

afterEach(() => {
  cleanup()
})

test('defaults to the first tab', () => {
  renderTabs()
  expect(screen.getByText('Overview panel')).toBeInTheDocument()
  expect(screen.queryByText('Rating panel')).not.toBeInTheDocument()
  expect(screen.getByRole('tab', { name: 'Overview' })).toHaveAttribute('aria-selected', 'true')
})

test('switching tabs shows the right panel and persists in the URL', () => {
  renderTabs()
  fireEvent.click(screen.getByRole('tab', { name: 'Rating' }))

  expect(screen.getByText('Rating panel')).toBeInTheDocument()
  expect(screen.queryByText('Overview panel')).not.toBeInTheDocument()
  expect(screen.getByTestId('search')).toHaveTextContent('tab=rating')
})

test('opens the tab named in the URL on mount', () => {
  renderTabs('/players/u1?tab=achievements')
  expect(screen.getByText('Achievements panel')).toBeInTheDocument()
  expect(screen.getByRole('tab', { name: 'Achievements' })).toHaveAttribute('aria-selected', 'true')
})

test('an unknown tab value falls back to the first tab', () => {
  renderTabs('/players/u1?tab=bogus')
  expect(screen.getByText('Overview panel')).toBeInTheDocument()
})
