import { skinUnlockSourceLabel, type CatalogSkinDto, type OwnedSkinDto, type SkinType, type SkinUnlockConditionDto, type SkinUnlockRuleDto } from '../api/skins'
import { useSkinAsset } from '../hooks/useSkinAsset'
import { Button } from './Button'

const categories: Array<{ type: SkinType; label: string }> = [
  { type: 'profile_background', label: 'Profile backgrounds' },
  { type: 'player_card_background', label: 'Player card backgrounds' },
  { type: 'avatar_frame', label: 'Avatar frames' },
  { type: 'display_picture', label: 'Display pictures' },
]

const cardWidthClasses: Record<SkinType, string> = {
  profile_background: 'w-full max-w-xl',
  player_card_background: 'w-full max-w-56',
  avatar_frame: 'w-full max-w-56',
  display_picture: 'w-full max-w-56',
}

type SkinPickerProps = {
  skins: OwnedSkinDto[]
  catalog?: CatalogSkinDto[]
  busyType: SkinType | null
  onEquip: (skin: OwnedSkinDto) => void
  onUnequip: (skinType: SkinType) => void
}

export function SkinPicker({ skins, catalog = [], busyType, onEquip, onUnequip }: SkinPickerProps) {
  const ownedByID = new Map(skins.map((skin) => [skin.id, skin]))
  const catalogIDs = new Set(catalog.map((skin) => skin.id))
  const allSkins = catalog.length > 0 ? [...catalog, ...skins.filter((skin) => !catalogIDs.has(skin.id))] : skins

  return (
    <div className="grid gap-8">
      {categories.map(({ type, label }) => {
        const items = allSkins.filter((skin) => skin.skin_type === type)
        return (
          <section key={type} className="grid gap-3" aria-label={label}>
            <div>
              <h3 className="font-mono text-xs font-medium uppercase tracking-[0.18em] text-spade-gold-light">{label}</h3>
            </div>
            {items.length === 0 ? (
              <p className="text-sm text-spade-gray-3">No cosmetics unlocked yet.</p>
            ) : (
              <div className="flex flex-wrap items-start gap-3">
                {items.map((skin) => {
                  const owned = ownedByID.get(skin.id)
                    const requirement = 'unlock_rules' in skin ? formatUnlockRules(skin.unlock_rules) : null
                  return (
                    <article
                      key={skin.id}
                      aria-label={`${skin.name} cosmetic`}
                      className={`${cardWidthClasses[skin.skin_type]} relative rounded-spade-lg border p-3 transition ${
                        owned?.equipped
                          ? 'border-spade-gold bg-spade-gold/10 shadow-[0_0_20px_rgba(212,175,55,0.12)]'
                          : owned
                            ? 'border-spade-cream/10 bg-spade-bg/40'
                            : 'border-spade-cream/10 bg-spade-bg/25 opacity-70'
                      }`}
                    >
                      <SkinPreview skin={skin} />
                      <div className="flex items-center justify-between gap-2">
                        <h4 className="text-sm font-medium text-spade-cream">{skin.name}</h4>
                        <span className="flex items-center gap-1.5 font-mono text-[10px] uppercase tracking-wider text-spade-gray-2">
                          {owned ? 'Owned' : 'Locked'}
                          {owned ? (
                            <details className="group relative font-sans normal-case tracking-normal">
                              <summary aria-label={`Unlock details for ${skin.name}`} className="grid size-5 cursor-pointer list-none place-items-center rounded-full border border-spade-cream/20 text-[10px] text-spade-gold-light transition hover:border-spade-gold/45 hover:text-spade-cream focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-spade-gold-light/60">
                                <span aria-hidden="true">i</span>
                              </summary>
                              <div className="absolute right-0 top-full z-20 mt-2 w-48 rounded-spade-md border border-spade-gold/30 bg-[#0b1b10] p-3 text-left text-xs font-normal leading-relaxed text-spade-cream shadow-[0_12px_28px_rgba(0,0,0,0.45)]">
                                  {requirement
                                    ? requirement
                                  : owned.source === 'starter'
                                    ? 'Available to every player as a starter cosmetic'
                                    : skinUnlockSourceLabel(owned.source)}
                              </div>
                            </details>
                          ) : null}
                        </span>
                      </div>
                      <p className="mt-1 min-h-10 text-xs text-spade-gray-3">{skin.description}</p>
                      {owned ? (
                        <Button
                          variant={owned.equipped ? 'secondary' : 'ghost'}
                          className="mt-3 w-full"
                          disabled={busyType === type}
                          onClick={() => owned.equipped ? onUnequip(type) : onEquip(owned)}
                        >
                          {owned.equipped ? 'Use default' : 'Equip'}
                        </Button>
                      ) : (
                        <details className="group relative mt-3">
                          <summary className="flex min-h-9 cursor-pointer list-none items-center justify-between rounded-spade-md border border-spade-cream/10 px-3 py-2 text-xs text-spade-gray-2 transition hover:border-spade-gold/35 hover:text-spade-cream focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-spade-gold-light/60">
                            <span>Unlock details</span>
                            <span aria-hidden="true" className="grid size-4 place-items-center rounded-full border border-spade-cream/20 font-mono text-[10px] text-spade-gold-light">i</span>
                          </summary>
                          <div className="absolute bottom-full left-0 z-20 mb-2 w-full rounded-spade-md border border-spade-gold/30 bg-[#0b1b10] p-3 text-xs leading-relaxed text-spade-cream shadow-[0_12px_28px_rgba(0,0,0,0.45)]">
                              {requirement
                                ? requirement
                              : 'Unlock requirement unavailable'}
                          </div>
                        </details>
                      )}
                    </article>
                  )
                })}
              </div>
            )}
          </section>
        )
      })}
    </div>
  )
}

function formatUnlockRules(rules: SkinUnlockRuleDto[]): string | null {
  const descriptions = rules.map(formatUnlockRule).filter((value): value is string => Boolean(value))
  return descriptions.length > 0 ? descriptions.join(' or ') : null
}

function formatUnlockRule(rule: SkinUnlockRuleDto): string | null {
  switch (rule.rule_type) {
    case 'achievement':
      return `Earn the ${rule.achievement.name} achievement`
    case 'minimum_level':
      return `Reach player level ${rule.minimum_level}`
    case 'login_streak':
      return `Log in on ${rule.login_streak_days} consecutive days`
    case 'event_check_in_count':
      return `Check in on ${rule.event_check_in_count} event ${rule.event_check_in_count === 1 ? 'day' : 'days'}`
    case 'game_condition': {
      const conditions = rule.conditions.map(formatUnlockCondition).filter((value): value is string => Boolean(value))
      return conditions.length > 0 ? conditions.join(' and ') : `Complete the ${rule.name} challenge`
    }
  }
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

function SkinPreview({ skin }: { skin: CatalogSkinDto | OwnedSkinDto }) {
  const assetURL = useSkinAsset(skin.id, skin.asset_key)
  if (skin.skin_type === 'profile_background') {
    return (
      <div className="mb-3 h-auto overflow-hidden rounded-spade-md bg-spade-green/30 p-1.5">
        <div
          aria-label={`${skin.name} profile background preview`}
          className="relative aspect-[20/7] w-full overflow-hidden rounded-spade-md border border-spade-cream/15 bg-spade-bg/50"
        >
          {assetURL ? <img src={assetURL} alt="" className="absolute inset-0 size-full object-contain" /> : null}
        </div>
      </div>
    )
  }
  if (skin.skin_type === 'player_card_background') {
    return (
      <div className="mb-3 h-auto overflow-hidden rounded-spade-md bg-spade-green/30 p-1.5">
        <div
          aria-label={`${skin.name} player card preview`}
          className="relative grid aspect-[6/7] w-full place-items-center overflow-hidden rounded-spade-md border border-spade-cream/15 bg-spade-bg/50"
        >
          {assetURL ? <img src={assetURL} alt="" className="absolute inset-0 size-full object-contain" /> : null}
        </div>
      </div>
    )
  }
  if (skin.skin_type === 'avatar_frame') {
    return (
      <div className="mb-3 h-auto overflow-hidden rounded-spade-md bg-spade-green/30 p-1.5">
        <div
          aria-label={`${skin.name} avatar frame preview`}
          className="relative grid aspect-square w-full place-items-center overflow-hidden rounded-spade-md bg-spade-bg/30"
        >
          {assetURL ? <img src={assetURL} alt="" className="pointer-events-none absolute inset-0 z-10 size-full object-contain" /> : null}
        </div>
      </div>
    )
  }
  return (
    <div className="mb-3 h-auto overflow-hidden rounded-spade-md bg-spade-green/30 p-1.5">
      <div
        aria-label={`${skin.name} display picture preview`}
        className="relative aspect-square w-full overflow-hidden rounded-spade-md bg-spade-green-mid"
      >
        {assetURL ? <img src={assetURL} alt="" className="size-full object-contain" /> : null}
      </div>
    </div>
  )
}
