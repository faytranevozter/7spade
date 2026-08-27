import type { SkinUnlockConditionDto, SkinUnlockRuleDto } from '../api/skins'

export function formatSkinUnlockRules(rules: SkinUnlockRuleDto[] | undefined): string | null {
  const descriptions = (rules ?? []).map(formatUnlockRule).filter((value): value is string => Boolean(value))
  return descriptions.length > 0 ? descriptions.join(' or ') : null
}

export function skinUnlockRuleLines(rule: SkinUnlockRuleDto): string[] {
  if (rule.rule_type === 'game_condition') {
    const conditions = (rule.conditions ?? []).map(formatUnlockCondition).filter((value): value is string => Boolean(value))
    if (conditions.length > 0) return conditions
  }
  const description = formatUnlockRule({ ...rule, event: undefined } as SkinUnlockRuleDto)
  return description ? [description] : []
}

function formatUnlockRule(rule: SkinUnlockRuleDto): string | null {
  let description: string | null
  switch (rule.rule_type) {
    case 'achievement':
      description = rule.achievement?.name ? `Earn the ${rule.achievement.name} achievement` : null
      break
    case 'minimum_level':
      description = rule.minimum_level !== undefined ? `Reach player level ${rule.minimum_level}` : null
      break
    case 'login_streak':
      description = rule.login_streak_days !== undefined ? `Log in on ${rule.login_streak_days} consecutive days` : null
      break
    case 'event_check_in_count':
      description = rule.event_check_in_count !== undefined
        ? `Check in on ${rule.event_check_in_count} event ${rule.event_check_in_count === 1 ? 'day' : 'days'}`
        : null
      break
    case 'game_condition': {
      const conditions = (rule.conditions ?? []).map(formatUnlockCondition).filter((value): value is string => Boolean(value))
      description = conditions.length > 0 ? conditions.join(' and ') : rule.name ? `Complete the ${rule.name} challenge` : null
      break
    }
  }
  return description && rule.event?.name ? `${description} during ${rule.event.name}` : description
}

function formatUnlockCondition(condition: SkinUnlockConditionDto): string | null {
  const comparison = formatComparison(condition.operator, condition.value)
  switch (condition.metric) {
    case 'is_winner': return condition.value === 'true' ? 'Win a completed game' : 'Finish a completed game without winning'
    case 'shared_win_count': return comparison && `Share a win with ${comparison} players`
    case 'penalty': return comparison && `Finish with ${comparison} penalty points`
    case 'games_played': return comparison && `Play ${comparison} games`
    case 'wins': return comparison && `Win ${comparison} games`
    case 'current_streak': return comparison && `Reach a win streak of ${comparison}`
    case 'current_top2_streak': return comparison && `Reach a top-two streak of ${comparison}`
    case 'first_place_count': return comparison && `Finish first in ${comparison} games`
    case 'zero_penalty_games': return comparison && `Complete ${comparison} zero-penalty games`
    case 'human_only_games': return comparison && `Complete ${comparison} human-only games`
    case 'all_zero_penalty': return condition.value === 'true' ? 'Complete a game where every player has zero penalty' : 'Complete a game where not every player has zero penalty'
    case 'ace_closed': return condition.value === 'true' ? 'Close an Ace during the game' : 'Complete a game without closing an Ace'
    case 'game_duration_seconds': return comparison && `Finish a game in ${comparison} seconds`
    default: return null
  }
}

function formatComparison(operator: SkinUnlockConditionDto['operator'], value: string): string | null {
  switch (operator) {
    case 'eq': return `exactly ${value}`
    case 'gte': return `at least ${value}`
    case 'lte': return `at most ${value}`
    case 'gt': return `more than ${value}`
    case 'lt': return `fewer than ${value}`
    default: return null
  }
}