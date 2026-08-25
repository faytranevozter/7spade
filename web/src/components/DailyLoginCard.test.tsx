import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { DailyLoginCard } from './DailyLoginCard'

const progress = {
  current_streak: 4,
  best_streak: 7,
  last_claim_date: '2026-08-24',
  claimed_today: false,
  new_skin_grants: [],
}

describe('DailyLoginCard', () => {
  it('offers the next streak day and claims it once', () => {
    const onClaim = vi.fn()
    render(<DailyLoginCard progress={progress} loading={false} claiming={false} error={null} onClaim={onClaim} onRetry={vi.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Claim day 5' }))
    expect(onClaim).toHaveBeenCalledOnce()
    expect(screen.getByText('Best: 7 days · resets at 00:00 UTC')).not.toBeNull()
  })

  it('disables an already claimed day', () => {
    render(<DailyLoginCard progress={{ ...progress, claimed_today: true }} loading={false} claiming={false} error={null} onClaim={vi.fn()} onRetry={vi.fn()} />)

    expect((screen.getByRole('button', { name: 'Claimed today' }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('allows a failed load to be retried', () => {
    const onRetry = vi.fn()
    render(<DailyLoginCard progress={null} loading={false} claiming={false} error="Could not load." onClaim={vi.fn()} onRetry={onRetry} />)

    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(onRetry).toHaveBeenCalledOnce()
  })
})
