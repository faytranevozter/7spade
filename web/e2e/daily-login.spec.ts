import { expect, test, type Page, type Route } from '@playwright/test'

const TOKEN_KEY = 'seven_spade_auth_token'

type LoginStreakResponse = {
  current_streak: number
  best_streak: number
  last_claim_date: string | null
  claimed_today: boolean
  newly_claimed?: boolean
  xp_delta?: number
  xp_after?: number
  level?: number
  has_login_streak_reward?: boolean
  next_reward_day?: number | null
  new_skin_grants: unknown[]
  timezone?: string
}

function jwt(isGuest: boolean): string {
  const encode = (value: object) => Buffer.from(JSON.stringify(value)).toString('base64url')
  return `${encode({ alg: 'none', typ: 'JWT' })}.${encode({ sub: 'e2e-user', display_name: 'E2E User', is_guest: isGuest })}.signature`
}

async function openLobby(page: Page, isGuest: boolean, streak?: LoginStreakResponse) {
  if (streak && !streak.timezone) streak.timezone = 'UTC'
  await page.addInitScript(({ key, token }) => {
    sessionStorage.setItem(key, token)
    localStorage.setItem('seven_spade_tutorial', 'completed')
  }, {
    key: TOKEN_KEY,
    token: jwt(isGuest),
  })

  await page.route('http://localhost:8080/**', async (route) => handleAPI(route, streak))
  await page.goto('/lobby')
  await expect(page.getByRole('heading', { name: 'Game lobby' })).toBeVisible()
}

async function handleAPI(route: Route, streak?: LoginStreakResponse) {
  const request = route.request()
  const path = new URL(request.url()).pathname

  if (path === '/me/login-streak' && request.method() === 'GET') {
    await route.fulfill({ json: streak })
    return
  }
  if (path === '/me/login-streak/claim' && request.method() === 'POST') {
    await route.fulfill({
      json: {
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
      },
    })
    return
  }

  const responses: Record<string, unknown> = {
    '/rooms': [],
    '/live-games': { games: [] },
    '/stats': { rating: 1200 },
    '/my/active-room': { active_room: null },
    '/friends': { friends: [], incoming: [], outgoing: [], blocked: [] },
  }
  await route.fulfill({ json: responses[path] ?? {} })
}

test.describe('Daily login lobby', () => {
  test('is hidden from guests', async ({ page }) => {
    await openLobby(page, true)

    await expect(page.getByText('Daily login')).toHaveCount(0)
    await expect(page.getByRole('button', { name: /Claim day/ })).toHaveCount(0)
  })

  test('claims the next streak day successfully', async ({ page }) => {
    await openLobby(page, false, {
      current_streak: 4,
      best_streak: 7,
      last_claim_date: '2026-08-24',
      claimed_today: false,
      has_login_streak_reward: true,
      next_reward_day: 7,
      new_skin_grants: [],
    })

    await expect(page.getByRole('button', { name: 'Claim daily reward' })).toBeVisible()
    const claimRequest = page.waitForRequest((request) =>
      request.url().endsWith('/me/login-streak/claim') && request.method() === 'POST')
    await page.getByRole('button', { name: 'Claim daily reward' }).click()
    await claimRequest

    await expect(page.getByRole('button', { name: 'Reward claimed' })).toBeDisabled()
    await expect(page.getByRole('dialog', { name: 'XP earned' })).toBeVisible()
    await expect(page.getByText('+30 XP')).toBeVisible()
    await expect(page.getByText('1,230 total XP · Level 4')).toBeVisible()
    await expect(page.getByText('Your streak is now 5 days.')).toBeVisible()
    await expect(page.getByText('5', { exact: true }).first()).toBeVisible()
  })

  test('shows a rolling week after seven streak days', async ({ page }) => {
    await openLobby(page, false, {
      current_streak: 10,
      best_streak: 12,
      last_claim_date: '2026-08-25',
      claimed_today: true,
      new_skin_grants: [],
    })

    await expect(page.getByText('Beyond one week')).toBeVisible()
    await expect(page.getByLabel('10-day streak; latest 7 days complete')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Claimed today' })).toBeDisabled()
  })

  test('offers day one when an expired streak has reset', async ({ page }) => {
    await openLobby(page, false, {
      current_streak: 0,
      best_streak: 8,
      last_claim_date: '2026-08-20',
      claimed_today: false,
      new_skin_grants: [],
    })

    await expect(page.getByText('0', { exact: true }).first()).toBeVisible()
    await expect(page.getByText('Best: 8 days · resets at 00:00 UTC')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Claim day 1' })).toBeVisible()
    await expect(page.getByLabel('0 of the last 7 streak days complete')).toBeVisible()
  })
})
