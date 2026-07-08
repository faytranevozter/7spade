import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { ToastStack } from './ToastStack'

afterEach(cleanup)

test('renders toast titles and bodies', () => {
  render(
    <ToastStack
      toasts={[
        { id: 1, tone: 'success', title: 'Copied', body: 'Invite link copied.' },
        { id: 2, tone: 'error', title: 'Failed', body: 'Try again.' },
      ]}
    />,
  )

  expect(screen.getByText('Copied')).toBeInTheDocument()
  expect(screen.getByText('Invite link copied.')).toBeInTheDocument()
  expect(screen.getByText('Failed')).toBeInTheDocument()
})
