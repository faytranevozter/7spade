import { useEffect, useState } from 'react'
import { skinUnlockSourceLabel, type CatalogSkinDto, type OwnedSkinDto, type SkinType } from '../api/skins'
import { useSkinAsset } from '../hooks/useSkinAsset'
import { Button } from './Button'
import { formatSkinUnlockRules, skinUnlockRuleLines } from './skinUnlockRequirements'

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
  const [openSkinID, setOpenSkinID] = useState<string | null>(null)
  const ownedByID = new Map(skins.map((skin) => [skin.id, skin]))
  const catalogIDs = new Set(catalog.map((skin) => skin.id))
  const allSkins = catalog.length > 0 ? [...catalog, ...skins.filter((skin) => !catalogIDs.has(skin.id))] : skins

  useEffect(() => {
    const closeOnOutsideInteraction = (event: PointerEvent) => {
      if (!(event.target as Element).closest('[data-skin-popover]')) {
        document.querySelectorAll<HTMLDetailsElement>('[data-skin-popover][open]').forEach((details) => {
          details.open = false
        })
        setOpenSkinID(null)
      }
    }
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        document.querySelectorAll<HTMLDetailsElement>('[data-skin-popover][open]').forEach((details) => {
          details.open = false
        })
        setOpenSkinID(null)
      }
    }
    document.addEventListener('pointerdown', closeOnOutsideInteraction)
    document.addEventListener('keydown', closeOnEscape)
    return () => {
      document.removeEventListener('pointerdown', closeOnOutsideInteraction)
      document.removeEventListener('keydown', closeOnEscape)
    }
  }, [])

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
                    const popoverOpen = openSkinID === skin.id
                    const setSkinPopoverOpen = (open: boolean) => {
                      setOpenSkinID((current) => open ? skin.id : current === skin.id ? null : current)
                    }
                  return (
                    <article
                      key={skin.id}
                      aria-label={`${skin.name} cosmetic`}
                        className={`${cardWidthClasses[skin.skin_type]} relative rounded-spade-lg border p-3 transition ${popoverOpen ? 'z-10' : 'z-0'} ${
                        owned?.equipped
                          ? 'border-spade-gold bg-spade-gold/10 shadow-[0_0_20px_rgba(212,175,55,0.12)]'
                          : owned
                            ? 'border-spade-cream/10 bg-spade-bg/40'
                              : 'border-spade-cream/10 bg-spade-bg/25'
                      }`}
                    >
                      <SkinPreview skin={skin} />
                      <div className="flex items-center justify-between gap-2">
                        <h4 className="text-sm font-medium text-spade-cream">{skin.name}</h4>
                        <span className="flex items-center gap-1.5 font-mono text-[10px] uppercase tracking-wider text-spade-gray-2">
                          {owned ? 'Owned' : 'Locked'}
                          {owned ? (
                              <SkinUnlockPopover
                                skin={skin}
                                open={popoverOpen}
                                  onOpenChange={setSkinPopoverOpen}
                                fallback={owned.source === 'starter' ? 'Available to every player as a starter cosmetic' : skinUnlockSourceLabel(owned.source)}
                                trigger="icon"
                              />
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
                          <SkinUnlockPopover
                            skin={skin}
                            open={popoverOpen}
                              onOpenChange={setSkinPopoverOpen}
                            fallback="Unlock requirement unavailable"
                            trigger="button"
                          />
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

type SkinUnlockPopoverProps = {
  skin: CatalogSkinDto | OwnedSkinDto
  open: boolean
  onOpenChange: (open: boolean) => void
  fallback: string
  trigger: 'icon' | 'button'
}

function SkinUnlockPopover({ skin, open, onOpenChange, fallback, trigger }: SkinUnlockPopoverProps) {
  const rules = 'unlock_rules' in skin ? skin.unlock_rules : null
  const content = rules && formatSkinUnlockRules(rules)
  return (
    <details data-skin-popover open={open} onToggle={(event) => onOpenChange(event.currentTarget.open)} className={`group relative font-sans normal-case tracking-normal ${trigger === 'button' ? 'mt-3' : ''}`}>
      <summary
        aria-label={trigger === 'icon' ? `Unlock details for ${skin.name}` : undefined}
        className={trigger === 'icon'
          ? 'grid size-5 cursor-pointer list-none place-items-center rounded-full border border-spade-cream/20 text-[10px] text-spade-gold-light transition hover:border-spade-gold/45 hover:text-spade-cream focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-spade-gold-light/60 select-none'
          : 'flex min-h-9 cursor-pointer list-none items-center justify-between rounded-spade-md border border-spade-cream/10 px-3 py-2 text-xs text-spade-gray-2 transition hover:border-spade-gold/35 hover:text-spade-cream focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-spade-gold-light/60 select-none'}
      >
        {trigger === 'button' ? <span className="select-none">Unlock details</span> : null}
        <span aria-hidden="true" className={trigger === 'button' ? 'grid size-4 place-items-center rounded-full border border-spade-cream/20 font-mono text-[10px] text-spade-gold-light' : ''}>i</span>
      </summary>
      <div className={`absolute z-20 w-72 max-w-[calc(100vw-2rem)] rounded-spade-md border border-spade-gold/30 bg-[#0b1b10] p-3 text-left text-xs font-normal leading-relaxed text-spade-cream shadow-[0_12px_28px_rgba(0,0,0,0.45)] ${trigger === 'button' ? 'bottom-full left-0 mb-2' : 'right-0 top-full mt-2'}`}>
        {content && rules ? <UnlockDetails rules={rules} /> : fallback}
      </div>
    </details>
  )
}

function UnlockDetails({ rules }: { rules: CatalogSkinDto['unlock_rules'] }) {
  return (
    <div className="grid gap-3">
      <p className="font-mono text-[10px] uppercase tracking-[0.16em] text-spade-gold-light">How to unlock</p>
      {rules.map((rule, ruleIndex) => (
        <div key={`${rule.rule_type}-${ruleIndex}`} className="grid gap-2">
          {ruleIndex > 0 ? <div className="flex items-center gap-2 text-[10px] uppercase tracking-wider text-spade-gray-3"><span className="h-px flex-1 bg-spade-cream/10" /><span>or</span><span className="h-px flex-1 bg-spade-cream/10" /></div> : null}
            <ol className="grid gap-2">
              {skinUnlockRuleLines(rule).map((line, lineIndex) => (
                <li key={line} className="grid grid-cols-[1.25rem_1fr] items-start gap-1.5">
                  <span aria-hidden="true" className="grid size-5 place-items-center rounded-full border border-spade-gold/25 bg-spade-gold/10 font-mono text-[9px] leading-none text-spade-gold-light">{lineIndex + 1}</span>
                  <span className="pt-0.5 leading-snug">{line}</span>
                </li>
              ))}
            </ol>
          {rule.event?.name ? (
            <div className="mt-1 rounded-spade-md border border-spade-gold/20 bg-spade-gold/5 px-2.5 py-2">
              <p className="font-mono text-[9px] uppercase tracking-[0.14em] text-spade-gray-3">Event exclusive</p>
              <p className="mt-0.5 font-medium text-spade-gold-light">{rule.event.name}</p>
            </div>
          ) : null}
        </div>
      ))}
    </div>
  )
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
