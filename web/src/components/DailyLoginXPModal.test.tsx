import { fireEvent, render, screen } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import { DailyLoginXPModal } from './DailyLoginXPModal'

it('presents the earned daily XP and progression total', () => {
  const onClose = vi.fn()
  render(<DailyLoginXPModal reward={{
    current_streak: 5,
    best_streak: 7,
    last_claim_date: '2026-08-25',
    claimed_today: true,
    newly_claimed: true,
    xp_delta: 30,
    xp_after: 1230,
    level: 4,
    has_login_streak_reward: true,
    next_reward_day: 7,
    new_skin_grants: [],
    app_timezone: 'UTC',
  }} onClose={onClose} />)

  expect(screen.getByText('+30 XP')).not.toBeNull()
  expect(screen.getByText('1,230 total XP · Level 4')).not.toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'Continue' }))
  expect(onClose).toHaveBeenCalledOnce()
})
