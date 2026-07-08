import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { Button } from './Button'

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

test('forwards button props and click handler', () => {
  const onClick = vi.fn()

  render(<Button onClick={onClick}>Join</Button>)
  fireEvent.click(screen.getByRole('button', { name: 'Join' }))

  expect(onClick).toHaveBeenCalledOnce()
})
