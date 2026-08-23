import type { SkinGrantDto } from '../api/auth'

const LOGIN_REWARDS_KEY = 'seven_spade_login_rewards'

export function storeLoginRewards(grants: SkinGrantDto[]): void {
  if (grants.length === 0) {
    sessionStorage.removeItem(LOGIN_REWARDS_KEY)
    return
  }
  sessionStorage.setItem(LOGIN_REWARDS_KEY, JSON.stringify(grants))
}

export function consumeLoginRewards(): SkinGrantDto[] {
  const value = sessionStorage.getItem(LOGIN_REWARDS_KEY)
  sessionStorage.removeItem(LOGIN_REWARDS_KEY)
  if (!value) return []
  try {
    const grants = JSON.parse(value)
    return Array.isArray(grants) ? grants : []
  } catch {
    return []
  }
}
