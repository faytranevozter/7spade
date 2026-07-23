import type { ReactNode } from 'react'
import type { UserStatsDto } from '../api/stats'
import { ProfileHero } from './ProfileHero'
import { ProfileTabs, type ProfileTab } from './ProfileTabs'

type ProfileViewProps = {
  displayName: string
  username?: string | null
  avatarUrl?: string | null
  stats: UserStatsDto | null
  showHeadlineStats?: boolean
  heroMeta?: ReactNode
  heroActions?: ReactNode
  tabs: ProfileTab[]
}

// ProfileView is the shared body for own + public profiles: hero card, then tabs.
export function ProfileView({
  displayName,
  username,
  avatarUrl,
  stats,
  showHeadlineStats = true,
  heroMeta,
  heroActions,
  tabs,
}: ProfileViewProps) {
  return (
    <div className="grid gap-4">
      <ProfileHero
        displayName={displayName}
        username={username}
        avatarUrl={avatarUrl}
        stats={stats}
        showHeadlineStats={showHeadlineStats}
        meta={heroMeta}
        actions={heroActions}
      />
      {tabs.length > 0 ? <ProfileTabs tabs={tabs} /> : null}
    </div>
  )
}
