import type { ReactNode } from 'react'
import type { UserStatsDto } from '../api/stats'
import { initialsForName } from '../game/cards'
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
}: ProfileHeroProps) {
  const handle = username ? `@${username}` : null

  return (
    <div className="rounded-spade-lg border border-spade-cream/10 bg-spade-bg/35 p-4 shadow-spade-card sm:p-5">
      <div className="flex flex-wrap items-start gap-4">
        <Avatar
          avatarUrl={avatarUrl}
          initials={initialsForName(displayName)}
          alt={displayName}
          sizeClass="size-20"
          className="text-2xl"
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
        <div className="mt-4 border-t border-spade-cream/10 pt-4">
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
