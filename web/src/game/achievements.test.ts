import { describe, expect, it } from 'vitest'
import { achievements, achievementMeta } from './achievements'

describe('achievements catalog', () => {
  it('has unique ids and non-empty name/description/icon', () => {
    const ids = achievements.map((a) => a.id)
    expect(new Set(ids).size).toBe(ids.length)
    for (const a of achievements) {
      expect(a.id).toBeTruthy()
      expect(a.name).toBeTruthy()
      expect(a.description).toBeTruthy()
      expect(a.icon).toBeTruthy()
    }
  })

  it('resolves metadata by id and returns undefined for unknown ids', () => {
    expect(achievementMeta('first_win')?.name).toBe('First Blood')
    expect(achievementMeta('definitely_not_real')).toBeUndefined()
  })
})
