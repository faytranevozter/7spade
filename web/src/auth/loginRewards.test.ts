import { beforeEach, describe, expect, it } from 'vitest'
import { consumeLoginRewards, storeLoginRewards } from './loginRewards'

const reward = {
  id: 'skin-1',
  skin_type: 'avatar_frame',
  name: 'Seven Day Streak Frame',
  description: 'reward',
  asset_key: 'skins/frames/gold-spade.svg',
  display_order: 130,
  source: 'login_streak:7',
}

describe('login rewards', () => {
  beforeEach(() => sessionStorage.clear())

  it('presents newly unlocked rewards once', () => {
    storeLoginRewards([reward])

    expect(consumeLoginRewards()).toEqual([reward])
    expect(consumeLoginRewards()).toEqual([])
  })

  it('clears empty and malformed handoffs', () => {
    storeLoginRewards([reward])
    storeLoginRewards([])
    expect(consumeLoginRewards()).toEqual([])

    sessionStorage.setItem('seven_spade_login_rewards', '{')
    expect(consumeLoginRewards()).toEqual([])
  })
})
