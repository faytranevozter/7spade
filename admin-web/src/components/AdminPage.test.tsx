import '@testing-library/jest-dom/vitest'
import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'
import { AdminPage, AdminPageHeader, AdminPanel } from './AdminPage'

test('renders standardized page and detail headers with explicit slots', () => {
  const { rerender } = render(
    <AdminPage labelledBy="catalog-title">
      <AdminPageHeader
        eyebrow="Content catalog"
        title="Achievements"
        titleId="catalog-title"
        description={<p>Review rewards.</p>}
        actions={<button type="button">Create</button>}
      />
      <AdminPanel className="catalog-panel">Catalog</AdminPanel>
    </AdminPage>,
  )

  expect(screen.getByRole('heading', { name: 'Achievements' })).toHaveClass(
    'text-admin-hero',
  )
  expect(screen.getByRole('button', { name: 'Create' })).toBeInTheDocument()
  expect(screen.getByText('Catalog')).toHaveClass('p-5', 'catalog-panel')

  rerender(
    <AdminPage>
      <AdminPageHeader
        eyebrow="Game record"
        title="Room 12"
        variant="detail"
        backLink={<a href="/rooms">Back</a>}
      />
    </AdminPage>,
  )
  expect(screen.getByRole('heading', { name: 'Room 12' })).toHaveClass(
    'text-admin-detail-hero',
  )
  expect(screen.getByRole('link', { name: 'Back' })).toBeInTheDocument()
})
