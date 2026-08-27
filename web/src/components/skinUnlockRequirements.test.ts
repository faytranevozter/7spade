import { expect, test } from 'vitest'
import type { SkinUnlockRuleDto } from '../api/skins'
import { formatSkinUnlockRules } from './skinUnlockRequirements'

test('names the event for an event-exclusive skin', () => {
  const rules: SkinUnlockRuleDto[] = [{
    rule_type: 'event_check_in_count',
    event_check_in_count: 3,
    event: {
      slug: 'summer-seven',
      name: 'Summer Seven',
      starts_at: '2026-08-01T00:00:00Z',
      ends_at: '2026-09-01T00:00:00Z',
    },
  }]

  expect(formatSkinUnlockRules(rules)).toBe('Check in on 3 event days during Summer Seven')
})

test('handles a partially upgraded game-condition rule without crashing', () => {
  const rule = { rule_type: 'game_condition', name: 'Perfect Hand' } as SkinUnlockRuleDto

  expect(formatSkinUnlockRules([rule])).toBe('Complete the Perfect Hand challenge')
})