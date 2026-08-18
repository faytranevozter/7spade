import type { ReactNode } from 'react'
import type { UserStatsDto } from '../api/stats'
import { initialsForName } from '../game/cards'
import { equippedSkin, type EquippedSkinDto } from '../api/skins'
import { useSkinAsset } from '../hooks/useSkinAsset'
import { Avatar } from './Avatar'
import { HeadlineStats } from './StatCards'

type ProfileHeroProps = {
  displayName: string
  username?: string | null
  avatarUrl?: string | null
  // When present, shows level/XP bar and optional headline strip.
  stats?: UserStatsDto | null
  showHeadlineStats?: boolean
  // Fallback line under the name when there is no username/level (e.g. guests).
  meta?: ReactNode
  actions?: ReactNode
  equippedSkins?: EquippedSkinDto[]
}

// ProfileHero is the shared identity card for /me and /players/:id — large avatar,
// name, @handle, level/XP, action buttons, and optional headline stats.
export function ProfileHero({
  displayName,
  username,
  avatarUrl,
  stats = null,
  showHeadlineStats = true,
  meta,
  actions,
  equippedSkins = [],
}: ProfileHeroProps) {
  const handle = username ? `@${username}` : null
  const backgroundSkin = equippedSkin(equippedSkins, 'profile_background')
  const frameSkin = equippedSkin(equippedSkins, 'avatar_frame')
  const displayPictureSkin = equippedSkin(equippedSkins, 'display_picture')
  const backgroundURL = useSkinAsset(backgroundSkin?.skin_id, backgroundSkin?.asset_key)
  const hasBackgroundSkin = Boolean(backgroundSkin)

  return (
    <div className="relative isolate overflow-hidden rounded-spade-lg border border-spade-cream/10 bg-spade-bg/35 p-4 shadow-spade-card sm:p-5">
      {!hasBackgroundSkin ? (
        <div
          aria-hidden="true"
          data-testid="default-profile-background"
          className="pointer-events-none absolute inset-0 z-0 bg-[radial-gradient(circle_at_88%_12%,rgba(212,175,55,0.18),transparent_30%),linear-gradient(135deg,rgba(31,92,63,0.28),transparent_55%)]"
        />
      ) : null}
      {backgroundURL ? (
        <div
          aria-hidden="true"
          data-testid="profile-background-skin"
          className="pointer-events-none absolute inset-0 z-0 bg-cover bg-center"
          style={{ backgroundImage: `url(${backgroundURL})` }}
        />
      ) : null}
      {!hasBackgroundSkin ? (
        <div aria-hidden="true" data-testid="default-profile-watermark" className="pointer-events-none absolute -right-8 -top-14 z-0 text-[11rem] leading-none text-spade-gold/[0.08] sm:-right-4 sm:-top-20 sm:text-[14rem]">
          ♠
        </div>
      ) : null}
      <div aria-hidden="true" className="pointer-events-none absolute bottom-0 left-0 z-0 h-px w-2/3 bg-gradient-to-r from-transparent via-spade-gold/60 to-transparent" />

      <div className="relative z-10 flex flex-wrap items-start gap-4">
        <Avatar
          avatarUrl={avatarUrl}
          initials={initialsForName(displayName)}
          alt={displayName}
          sizeClass="size-20"
          className="text-2xl"
          displayPictureAssetKey={displayPictureSkin?.asset_key}
          displayPictureSkinId={displayPictureSkin?.skin_id}
          frameAssetKey={frameSkin?.asset_key}
          frameSkinId={frameSkin?.skin_id}
        />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-start gap-3">
            <div className="min-w-0 flex-1 grid gap-1.5">
              <p className="truncate text-xl font-medium text-spade-cream sm:text-2xl">{displayName}</p>
              {stats ? (
                <>
                  <div className="flex flex-wrap items-center gap-2 text-sm text-spade-gray-2">
                    {handle ? <span className="font-mono text-spade-gray-3">{handle}</span> : null}
                    {handle ? <span className="text-spade-gray-4" aria-hidden="true">·</span> : null}
                    <span className="rounded-spade-md bg-spade-gold/20 px-2 py-0.5 font-mono text-[11px] font-medium text-spade-gold-light">
                      Lv {stats.level}
                    </span>
                  </div>
                  <LevelProgress stats={stats} />
                </>
              ) : meta ? (
                <div className="text-sm text-spade-gray-3">{meta}</div>
              ) : handle ? (
                <p className="font-mono text-sm text-spade-gray-3">{handle}</p>
              ) : null}
            </div>
            {actions ? <div className="flex flex-wrap gap-2">{actions}</div> : null}
          </div>
        </div>
      </div>
      {stats && showHeadlineStats ? (
        <div className="relative z-10 mt-4 border-t border-spade-cream/10 pt-4">
          <HeadlineStats stats={stats} />
        </div>
      ) : null}
    </div>
  )
}

// LevelProgress renders total XP and a bar toward the next level.
function LevelProgress({ stats }: { stats: UserStatsDto }) {
  const span = stats.xp_for_next_level
  const pct = span > 0 ? Math.min(100, Math.round((stats.xp_into_level / span) * 100)) : 100
  return (
    <div className="grid max-w-xs gap-1">
      <div
        className="h-2 w-full overflow-hidden rounded-full bg-spade-cream/12"
        role="progressbar"
        aria-valuenow={pct}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label={`Level progress ${pct}%`}
      >
        <div className="h-full rounded-full bg-spade-gold" style={{ width: `${pct}%` }} />
      </div>
      <span className="font-mono text-[10px] text-spade-gray-3">
        {stats.xp.toLocaleString()} XP · {stats.xp_to_next_level.toLocaleString()} to next
      </span>
    </div>
  )
}
