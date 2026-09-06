import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { ApiError } from '../api/client'
import { claimEventCheckIn, getEvent, type EventDetail } from '../api/events'
import { Button } from '../components/Button'
import { SceneShell } from '../components/SceneShell'
import { formatSkinUnlockRules } from '../components/skinUnlockRequirements'
import { useSkinAsset } from '../hooks/useSkinAsset'
import { useAuth } from '../hooks/useAuth'
import { decodeJwtClaims } from '../auth/claims'

type EventSkinReward = EventDetail['skin_rewards'][number]
type EventSkinType = EventSkinReward['skin']['skin_type']

const rewardCategories: Array<{ type: EventSkinType; label: string }> = [
  { type: 'profile_background', label: 'Profile backgrounds' },
  { type: 'player_card_background', label: 'Player card backgrounds' },
  { type: 'avatar_frame', label: 'Avatar frames' },
  { type: 'display_picture', label: 'Display pictures' },
]

const rewardPreviewClasses: Record<EventSkinType, string> = {
  profile_background: 'aspect-[20/7]',
  player_card_background: 'aspect-[6/7]',
  avatar_frame: 'aspect-square',
  display_picture: 'aspect-square',
}

const rewardCardWidthClasses: Record<EventSkinType, string> = {
  profile_background: 'w-full max-w-xl',
  player_card_background: 'w-full max-w-56',
  avatar_frame: 'w-full max-w-56',
  display_picture: 'w-full max-w-56',
}

export function EventPage() {
  const { slug = '' } = useParams()
  const { token } = useAuth()
  const isRegistered = Boolean(token) && !decodeJwtClaims(token).isGuest
  const [detail, setDetail] = useState<EventDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [claiming, setClaiming] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [claimNotice, setClaimNotice] = useState<string | null>(null)
  const [openRewardID, setOpenRewardID] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    Promise.resolve()
      .then(() => {
        if (cancelled) return null
        setLoading(true)
        setError(null)
        return getEvent(token, slug)
      })
      .then((value) => { if (!cancelled && value) setDetail(value) })
      .catch((err: unknown) => { if (!cancelled) setError(err instanceof ApiError ? err.message : 'Failed to load event') })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [slug, token])

  useEffect(() => {
    const closePopover = () => {
      document.querySelectorAll<HTMLDetailsElement>('[data-event-skin-popover][open]').forEach((details) => {
        details.open = false
      })
      setOpenRewardID(null)
    }
    const closeOnOutsideInteraction = (event: PointerEvent) => {
      if (!(event.target as Element).closest('[data-event-skin-popover]')) closePopover()
    }
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') closePopover()
    }
    document.addEventListener('pointerdown', closeOnOutsideInteraction)
    document.addEventListener('keydown', closeOnEscape)
    return () => {
      document.removeEventListener('pointerdown', closeOnOutsideInteraction)
      document.removeEventListener('keydown', closeOnEscape)
    }
  }, [])

  const claim = async () => {
    setClaiming(true)
    setError(null)
    setClaimNotice(null)
    try {
      const result = await claimEventCheckIn(token, slug)
      if (result.newly_claimed) {
        const skinText = result.skin_grants.length
          ? ` Unlocked ${result.skin_grants.map((skin) => skin.name).join(', ')}.`
          : ''
        setClaimNotice(`Claimed ${result.xp_delta} XP. You are now level ${result.level}.${skinText}`)
      } else {
        setClaimNotice('Today’s event reward was already claimed.')
      }
      setDetail(await getEvent(token, slug))
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to claim check-in')
    } finally {
      setClaiming(false)
    }
  }

  if (loading) return <SceneShell title="Event" eyebrow="Loading"><p className="text-spade-gray-2">Preparing event details...</p></SceneShell>
  if (!detail) return <SceneShell title="Event unavailable" eyebrow="Events"><p className="text-spade-gray-2">{error ?? 'This event could not be found.'}</p></SceneShell>

  const event = detail.event
  const skinRewards = detail.skin_rewards ?? []
  return (
    <SceneShell title={event.name} eyebrow={`${event.status} event`}>
      <div className="mx-auto grid w-full max-w-6xl gap-6 pb-12">
        <section className="relative overflow-hidden rounded-spade-xl border border-spade-gold/25 bg-spade-green-dark p-6 sm:p-10">
          <div className="absolute inset-y-0 right-0 w-1/2 bg-[radial-gradient(circle_at_center,rgba(212,175,55,0.16),transparent_65%)]" />
          <div className="relative max-w-3xl">
            <p className="font-mono text-xs uppercase tracking-[0.2em] text-spade-gold-light">{formatDate(event.starts_at)} - {formatDate(event.ends_at)}</p>
            <p className="mt-5 whitespace-pre-line leading-relaxed text-spade-gray-2">{event.description}</p>
          </div>
        </section>

        {event.daily_login.enabled ? (
          <section className="grid gap-4 rounded-spade-xl border border-spade-cream/10 bg-spade-bg/55 p-6 sm:grid-cols-[1fr_auto] sm:items-center">
            <div>
              <p className="font-mono text-xs uppercase tracking-[0.18em] text-spade-gold-light">Event attendance</p>
              <h2 className="mt-2 text-2xl font-semibold text-spade-cream">{detail.check_in.count} daily check-ins</h2>
              <p className="mt-1 text-sm text-spade-gray-2">Missing a day does not reset your progress.</p>
              <p className="mt-2 font-mono text-[11px] uppercase tracking-wider text-spade-gray-3">Resets at 00:00 {formatAppTimezone(event.app_timezone)}</p>
            </div>
            {event.status === 'active' ? (
              isRegistered ? <Button disabled={claiming || detail.check_in.claimed_today} onClick={claim}>{detail.check_in.claimed_today ? 'Checked in today' : claiming ? 'Claiming...' : `Claim today · +${event.daily_login.xp_per_claim} XP`}</Button>
                : <Link className="rounded-spade-md bg-spade-gold px-5 py-3 text-center font-medium text-spade-bg" to="/auth">Sign in to check in</Link>
            ) : <p className="font-mono text-xs uppercase tracking-wider text-spade-gray-3">Check-ins {event.status === 'upcoming' ? 'open when the event starts' : 'are closed'}</p>}
          </section>
        ) : null}

        {claimNotice ? <p role="status" className="rounded-spade-md border border-spade-gold/35 bg-spade-gold/10 p-3 text-sm text-spade-cream">{claimNotice}</p> : null}
        {error ? <p role="alert" className="rounded-spade-md border border-spade-red/35 bg-spade-red/10 p-3 text-sm text-spade-cream">{error}</p> : null}

        <section>
          <div className="mb-4">
            <p className="font-mono text-xs uppercase tracking-[0.18em] text-spade-gold-light">Limited rewards</p>
            <h2 className="mt-2 text-2xl font-semibold text-spade-cream">Event skins</h2>
          </div>
          {skinRewards.length ? <div className="grid gap-8">{rewardCategories.map((category) => {
            const rewards = skinRewards.filter((reward) => reward.skin.skin_type === category.type)
            if (rewards.length === 0) return null
            return <section key={category.type} aria-label={category.label} className="grid gap-3">
              <h3 className="font-mono text-xs font-medium uppercase tracking-[0.18em] text-spade-gold-light">{category.label}</h3>
              <div className="flex flex-wrap items-start gap-3">{rewards.map((reward) => (
                <RewardCard
                  key={reward.skin.id}
                  reward={reward}
                  popoverOpen={openRewardID === reward.skin.id}
                  onPopoverOpenChange={(open) => {
                    setOpenRewardID((current) => open ? reward.skin.id : current === reward.skin.id ? null : current)
                  }}
                />
              ))}</div>
            </section>
          })}</div>
            : <p className="rounded-spade-lg border border-spade-cream/10 p-5 text-sm text-spade-gray-2">No event rewards are available in this event phase.</p>}
        </section>
      </div>
    </SceneShell>
  )
}

function RewardCard({ reward, popoverOpen, onPopoverOpenChange }: {
  reward: EventSkinReward
  popoverOpen: boolean
  onPopoverOpenChange: (open: boolean) => void
}) {
  const assetURL = useSkinAsset(reward.skin.id, reward.skin.asset_key)
  const { progress, target } = reward.requirement
  const hasProgress = progress !== undefined && target !== undefined
  const percent = hasProgress && target > 0 ? Math.min(100, Math.round((progress / target) * 100)) : 0
  const remaining = hasProgress ? Math.max(0, target - progress) : null
  const status = reward.owned
    ? 'Unlocked and owned'
    : reward.requirement.completed
      ? 'Requirement completed'
      : remaining === null
        ? 'Complete this challenge during the event'
        : remaining === 1
          ? '1 step remaining'
          : `${remaining} steps remaining`
  const unlockDetails = formatSkinUnlockRules(reward.skin.unlock_rules) ?? reward.requirement.description

  return <article aria-label={`${reward.skin.name} event reward`} className={`${rewardCardWidthClasses[reward.skin.skin_type]} relative rounded-spade-lg border border-spade-cream/10 bg-spade-bg p-3 ${popoverOpen ? 'z-10' : 'z-0'}`}>
    <div className="mb-3 overflow-hidden rounded-spade-md bg-spade-green/30 p-1.5">
      <div className={`relative grid w-full place-items-center overflow-hidden rounded-spade-md border border-spade-cream/15 bg-spade-bg/50 ${rewardPreviewClasses[reward.skin.skin_type]}`}>
        {assetURL ? <img src={assetURL} alt={`${reward.skin.name} preview`} className="absolute inset-0 size-full object-contain" /> : <span aria-hidden="true" className="text-5xl text-spade-gold">♠</span>}
      </div>
    </div>
    <h4 className="text-sm font-medium text-spade-cream">{reward.skin.name}</h4>
    <p className="mt-1 min-h-10 text-xs text-spade-gray-3">{reward.skin.description}</p>
    {reward.owned ? (
      <Button className="mt-3 w-full" variant="secondary" disabled>Owned</Button>
    ) : (
      <details data-event-skin-popover open={popoverOpen} onToggle={(event) => onPopoverOpenChange(event.currentTarget.open)} className="group relative mt-3">
        <summary className="flex min-h-9 cursor-pointer list-none items-center justify-between rounded-spade-md border border-spade-cream/10 px-3 py-2 text-xs text-spade-gray-2 transition hover:border-spade-gold/35 hover:text-spade-cream focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-spade-gold-light/60 select-none">
          <span>Unlock details</span>
          <span aria-hidden="true" className="grid size-4 place-items-center rounded-full border border-spade-cream/20 font-mono text-[10px] text-spade-gold-light">i</span>
        </summary>
        <div className="absolute bottom-full left-0 z-10 mb-2 w-72 max-w-[calc(100vw-2rem)] rounded-spade-md border border-spade-gold/30 bg-[#0b1b10] p-3 text-xs leading-relaxed text-spade-cream shadow-[0_12px_28px_rgba(0,0,0,0.45)]">
          <p className="font-mono text-[10px] uppercase tracking-wider text-spade-gold-light">How to unlock</p>
          <p className="mt-2 text-sm">{unlockDetails}</p>
          <div className="mt-3 flex items-center justify-between gap-2 text-xs">
            <span className="text-spade-gray-2">{status}</span>
            {hasProgress ? <span className="font-mono text-spade-gold-light">{Math.min(progress, target)}/{target}</span> : null}
          </div>
          {hasProgress ? <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-spade-bg" role="progressbar" aria-label={`${reward.skin.name} unlock progress`} aria-valuemin={0} aria-valuemax={target} aria-valuenow={Math.min(progress, target)}><div className="h-full bg-spade-gold transition-[width]" style={{ width: `${percent}%` }} /></div> : null}
        </div>
      </details>
    )}
  </article>
}

function formatAppTimezone(value: string): string {
  return value === 'UTC' ? 'UTC' : `UTC ${value}`
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}
