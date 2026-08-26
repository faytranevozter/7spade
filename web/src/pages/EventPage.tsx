import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { ApiError } from '../api/client'
import { claimEventCheckIn, getEvent, type EventDetail } from '../api/events'
import { Button } from '../components/Button'
import { SceneShell } from '../components/SceneShell'
import { useSkinAsset } from '../hooks/useSkinAsset'
import { useAuth } from '../hooks/useAuth'
import { decodeJwtClaims } from '../auth/claims'

export function EventPage() {
  const { slug = '' } = useParams()
  const { token } = useAuth()
  const isRegistered = Boolean(token) && !decodeJwtClaims(token).isGuest
  const [detail, setDetail] = useState<EventDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [claiming, setClaiming] = useState(false)
  const [error, setError] = useState<string | null>(null)

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

  const claim = async () => {
    setClaiming(true)
    setError(null)
    try {
      await claimEventCheckIn(token, slug)
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

        <section className="grid gap-4 rounded-spade-xl border border-spade-cream/10 bg-spade-bg/55 p-6 sm:grid-cols-[1fr_auto] sm:items-center">
          <div>
            <p className="font-mono text-xs uppercase tracking-[0.18em] text-spade-gold-light">Event attendance</p>
            <h2 className="mt-2 text-2xl font-semibold text-spade-cream">{detail.check_in.count} daily check-ins</h2>
            <p className="mt-1 text-sm text-spade-gray-2">Missing a day does not reset your progress.</p>
            <p className="mt-2 font-mono text-[11px] uppercase tracking-wider text-spade-gray-3">Resets at 00:00 {formatAppTimezone(event.app_timezone)}</p>
          </div>
          {event.status === 'active' ? (
            isRegistered ? <Button disabled={claiming || detail.check_in.claimed_today} onClick={claim}>{detail.check_in.claimed_today ? 'Checked in today' : claiming ? 'Claiming...' : 'Claim today'}</Button>
              : <Link className="rounded-spade-md bg-spade-gold px-5 py-3 text-center font-medium text-spade-bg" to="/auth">Sign in to check in</Link>
          ) : <p className="font-mono text-xs uppercase tracking-wider text-spade-gray-3">Check-ins {event.status === 'upcoming' ? 'open when the event starts' : 'are closed'}</p>}
        </section>

        {error ? <p role="alert" className="rounded-spade-md border border-spade-red/35 bg-spade-red/10 p-3 text-sm text-spade-cream">{error}</p> : null}

        <section>
          <div className="mb-4">
            <p className="font-mono text-xs uppercase tracking-[0.18em] text-spade-gold-light">Limited rewards</p>
            <h2 className="mt-2 text-2xl font-semibold text-spade-cream">Event skins</h2>
          </div>
          {detail.skin_rewards.length ? <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">{detail.skin_rewards.map((reward) => <RewardCard key={reward.skin.id} reward={reward} />)}</div>
            : <p className="rounded-spade-lg border border-spade-cream/10 p-5 text-sm text-spade-gray-2">No event rewards are available in this event phase.</p>}
        </section>
      </div>
    </SceneShell>
  )
}

function RewardCard({ reward }: { reward: EventDetail['skin_rewards'][number] }) {
  const assetURL = useSkinAsset(reward.skin.id, reward.skin.asset_key)
  const { progress, target } = reward.requirement
  const hasProgress = progress !== undefined && target !== undefined
  const percent = hasProgress && target > 0 ? Math.min(100, Math.round((progress / target) * 100)) : 0
  const remaining = hasProgress ? Math.max(0, target - progress) : null
  const typeLabel = skinTypeLabel(reward.skin.skin_type)
  const status = reward.owned
    ? 'Unlocked and owned'
    : reward.requirement.completed
      ? 'Requirement completed'
      : remaining === null
        ? 'Complete this challenge during the event'
        : remaining === 1
          ? '1 step remaining'
          : `${remaining} steps remaining`
  const ownershipLabel = reward.owned ? 'Owned' : reward.requirement.completed ? 'Ready' : 'Locked'

  return <article aria-label={`${reward.skin.name} event reward`} className="overflow-hidden rounded-spade-lg border border-spade-gold/20 bg-spade-green-dark/70">
    <div className="relative grid aspect-[4/3] place-items-center bg-spade-bg/45 p-5">
      <span className="absolute left-3 top-3 rounded-full border border-spade-gold/25 bg-spade-bg/80 px-2.5 py-1 font-mono text-[10px] uppercase tracking-wider text-spade-gold-light">{typeLabel}</span>
      {assetURL ? <img src={assetURL} alt={`${reward.skin.name} preview`} className="size-full object-contain" /> : <span aria-hidden="true" className="text-5xl text-spade-gold">♠</span>}
    </div>
    <div className="p-4">
      <div className="flex items-start justify-between gap-3">
        <div><h3 className="font-medium text-spade-cream">{reward.skin.name}</h3><p className="mt-1 text-xs leading-relaxed text-spade-gray-3">{reward.skin.description}</p></div>
        <span className={`shrink-0 rounded-full px-2 py-1 font-mono text-[10px] uppercase tracking-wider ${reward.owned ? 'bg-spade-gold/15 text-spade-gold-light' : 'bg-spade-bg text-spade-gray-2'}`}>{ownershipLabel}</span>
      </div>
      <div className="mt-4 rounded-spade-md border border-spade-cream/10 bg-spade-bg/35 p-3">
        <p className="font-mono text-[10px] uppercase tracking-wider text-spade-gray-3">How to unlock</p>
        <p className="mt-1 text-sm text-spade-cream">{reward.requirement.description}</p>
        <div className="mt-3 flex items-center justify-between gap-2 text-xs"><span className="text-spade-gray-2">{status}</span>{hasProgress ? <span className="font-mono text-spade-gold-light">{Math.min(progress, target)}/{target}</span> : null}</div>
        {hasProgress ? <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-spade-bg" role="progressbar" aria-label={`${reward.skin.name} unlock progress`} aria-valuemin={0} aria-valuemax={target} aria-valuenow={Math.min(progress, target)}><div className="h-full bg-spade-gold transition-[width]" style={{ width: `${percent}%` }} /></div> : null}
      </div>
    </div>
  </article>
}

function skinTypeLabel(skinType: EventDetail['skin_rewards'][number]['skin']['skin_type']): string {
  return ({
    profile_background: 'Profile background',
    player_card_background: 'Player card background',
    avatar_frame: 'Avatar frame',
    display_picture: 'Display picture',
  })[skinType]
}

function formatAppTimezone(value: string): string {
  return value === 'UTC' ? 'UTC' : `UTC ${value}`
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}
