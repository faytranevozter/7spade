import type { DailyLoginRewardDto, SkinGrantDto } from '../api/auth'

const LOGIN_REWARDS_KEY = 'seven_spade_login_rewards'
const LOGIN_XP_REWARD_KEY = 'seven_spade_login_xp_reward'

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

export function storeLoginXPReward(reward: Partial<DailyLoginRewardDto>): void {
  if (!reward.newly_claimed || !reward.xp_delta) {
    sessionStorage.removeItem(LOGIN_XP_REWARD_KEY)
    return
  }
  sessionStorage.setItem(LOGIN_XP_REWARD_KEY, JSON.stringify(reward))
}

export function consumeLoginXPReward(): DailyLoginRewardDto | null {
  const value = sessionStorage.getItem(LOGIN_XP_REWARD_KEY)
  sessionStorage.removeItem(LOGIN_XP_REWARD_KEY)
  if (!value) return null
  try {
    const reward = JSON.parse(value) as DailyLoginRewardDto
    return reward.newly_claimed && reward.xp_delta > 0 ? reward : null
  } catch {
    return null
  }
}
