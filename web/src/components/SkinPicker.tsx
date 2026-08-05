import { type OwnedSkinDto, type SkinType } from '../api/skins'
import { useSkinAsset } from '../hooks/useSkinAsset'
import { Button } from './Button'

const categories: Array<{ type: SkinType; label: string }> = [
  { type: 'profile_background', label: 'Profile backgrounds' },
  { type: 'avatar_frame', label: 'Avatar frames' },
  { type: 'display_picture', label: 'Display pictures' },
]

type SkinPickerProps = {
  skins: OwnedSkinDto[]
  busyType: SkinType | null
  onEquip: (skin: OwnedSkinDto) => void
  onUnequip: (skinType: SkinType) => void
}

export function SkinPicker({ skins, busyType, onEquip, onUnequip }: SkinPickerProps) {
  return (
    <div className="grid gap-6">
      {categories.map(({ type, label }) => {
        const items = skins.filter((skin) => skin.skin_type === type)
        const equipped = items.find((skin) => skin.equipped)
        return (
          <section key={type} className="grid gap-3" aria-label={label}>
            <div className="flex flex-wrap items-baseline justify-between gap-2">
              <h3 className="font-mono text-xs font-medium uppercase tracking-[0.18em] text-spade-gold-light">{label}</h3>
              {equipped ? (
                <Button variant="ghost" disabled={busyType === type} onClick={() => onUnequip(type)}>Use default</Button>
              ) : null}
            </div>
            {items.length === 0 ? (
              <p className="text-sm text-spade-gray-3">No cosmetics unlocked yet.</p>
            ) : (
              <div className="grid gap-3 sm:grid-cols-3">
                {items.map((skin) => (
                    <article
                      key={skin.id}
                      className={`overflow-hidden rounded-spade-lg border p-3 transition ${
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
                        disabled={busyType === type || skin.equipped}
                        onClick={() => onEquip(skin)}
                      >
                        {skin.equipped ? 'Equipped' : 'Equip'}
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
  return (
    <div className="relative mb-3 grid h-20 place-items-center overflow-hidden rounded-spade-md bg-spade-green/30">
      {assetURL ? <img src={assetURL} alt="" className="size-full object-cover" /> : null}
      {skin.skin_type === 'avatar_frame' ? <span className="absolute grid size-12 place-items-center rounded-full border-4 border-spade-gold text-lg text-spade-cream">♠</span> : null}
      {skin.skin_type === 'display_picture' ? <span className="absolute grid size-12 place-items-center rounded-full bg-spade-gold text-xl text-spade-bg">♠</span> : null}
    </div>
  )
}
