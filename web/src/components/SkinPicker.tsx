import { type OwnedSkinDto, type SkinType } from '../api/skins'
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
  busyType: SkinType | null
  onEquip: (skin: OwnedSkinDto) => void
  onUnequip: (skinType: SkinType) => void
}

export function SkinPicker({ skins, busyType, onEquip, onUnequip }: SkinPickerProps) {
  return (
    <div className="grid gap-8">
      {categories.map(({ type, label }) => {
        const items = skins.filter((skin) => skin.skin_type === type)
        return (
          <section key={type} className="grid gap-3" aria-label={label}>
            <div>
              <h3 className="font-mono text-xs font-medium uppercase tracking-[0.18em] text-spade-gold-light">{label}</h3>
            </div>
            {items.length === 0 ? (
              <p className="text-sm text-spade-gray-3">No cosmetics unlocked yet.</p>
            ) : (
              <div className="flex flex-wrap items-start gap-3">
                {items.map((skin) => (
                    <article
                      key={skin.id}
                      aria-label={`${skin.name} cosmetic`}
                      className={`${cardWidthClasses[skin.skin_type]} overflow-hidden rounded-spade-lg border p-3 transition ${
                        skin.equipped
                          ? 'border-spade-gold bg-spade-gold/10 shadow-[0_0_20px_rgba(212,175,55,0.12)]'
                          : 'border-spade-cream/10 bg-spade-bg/40'
                      }`}
                    >
                      <SkinPreview skin={skin} />
                      <h4 className="text-sm font-medium text-spade-cream">{skin.name}</h4>
                      <p className="mt-1 min-h-10 text-xs text-spade-gray-3">{skin.description}</p>
                      <Button
                        variant={skin.equipped ? 'secondary' : 'ghost'}
                        className="mt-3 w-full"
                        disabled={busyType === type}
                        onClick={() => skin.equipped ? onUnequip(type) : onEquip(skin)}
                      >
                        {skin.equipped ? 'Use default' : 'Equip'}
                      </Button>
                    </article>
                  ))}
              </div>
            )}
          </section>
        )
      })}
    </div>
  )
}

function SkinPreview({ skin }: { skin: OwnedSkinDto }) {
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
