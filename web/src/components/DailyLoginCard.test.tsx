import { fireEvent, render, screen, within } from '@testing-library/react'
import '@testing-library/jest-dom/vitest'
import { describe, expect, it, vi } from 'vitest'
import { DailyLoginCard } from './DailyLoginCard'

const progress = {
  enabled: true,
  current_streak: 4,
  best_streak: 7,
  last_claim_date: '2026-08-24',
  claimed_today: false,
  newly_claimed: false,
  xp_delta: 0,
  xp_after: 100,
  level: 2,
  has_login_streak_reward: false,
  next_reward_day: null,
  new_skin_grants: [],
  app_timezone: 'UTC',
}

describe('DailyLoginCard', () => {
  it('renders nothing when daily login is disabled', () => {
    const { container } = render(<DailyLoginCard progress={{ ...progress, enabled: false }} loading={false} claiming={false} error={null} onClaim={vi.fn()} onRetry={vi.fn()} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('offers the next streak day and claims it once', () => {
    const onClaim = vi.fn()
    render(<DailyLoginCard progress={progress} loading={false} claiming={false} error={null} onClaim={onClaim} onRetry={vi.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Claim day 5' }))
    expect(onClaim).toHaveBeenCalledOnce()
    expect(screen.getByText('Best: 7 days · resets at 00:00 UTC')).not.toBeNull()
  })

  it('uses reward styling and copy when a login streak rule exists', () => {
    const onClaim = vi.fn()
    render(<DailyLoginCard progress={{ ...progress, has_login_streak_reward: true, next_reward_day: 7 }} loading={false} claiming={false} error={null} onClaim={onClaim} onRetry={vi.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Claim daily reward' }))
    expect(onClaim).toHaveBeenCalledOnce()
    expect(screen.getByText('Streak cosmetics active')).not.toBeNull()
    expect(screen.getByText('Next cosmetic at day 7')).not.toBeNull()
  })

  it('disables an already claimed day', () => {
    render(<DailyLoginCard progress={{ ...progress, claimed_today: true }} loading={false} claiming={false} error={null} onClaim={vi.fn()} onRetry={vi.fn()} />)

    expect((screen.getByRole('button', { name: 'Claimed today' }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('shows a rolling week for streaks longer than seven days', () => {
    render(<DailyLoginCard progress={{ ...progress, current_streak: 10, best_streak: 10, claimed_today: true }} loading={false} claiming={false} error={null} onClaim={vi.fn()} onRetry={vi.fn()} />)

    expect(screen.getByText('Beyond one week')).not.toBeNull()
    const strip = screen.getByLabelText('10-day streak; latest 7 days complete')
    expect(within(strip).getByText('4')).not.toBeNull()
    expect(within(strip).getByText('10')).not.toBeNull()
  })

  it('allows a failed load to be retried', () => {
    const onRetry = vi.fn()
    render(<DailyLoginCard progress={null} loading={false} claiming={false} error="Could not load." onClaim={vi.fn()} onRetry={onRetry} />)

    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(onRetry).toHaveBeenCalledOnce()
  })
})
