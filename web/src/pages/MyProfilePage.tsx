import { type FormEvent, useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { ApiError } from '../api/client'
import {
  AuthApiError,
  cancelDeletion,
  deleteAccount,
  getMe,
  updateDisplayName,
  type MeResponse,
} from '../api/auth'
import { getMyStats, getRatingHistory, type RatingEventDto, type UserStatsDto } from '../api/stats'
import { getUserAchievements, type AchievementDto, type EarnedAchievementDto } from '../api/achievements'
import { BadgeGrid } from '../components/BadgeGrid'
import { Button } from '../components/Button'
import { Modal } from '../components/Modal'
import { ProfileView } from '../components/ProfileView'
import { ProviderBadge } from '../components/ProviderBadge'
import { RatingHistory } from '../components/RatingHistory'
import { SceneShell } from '../components/SceneShell'
import { StatCards } from '../components/StatCards'
import { useAuth } from '../hooks/useAuth'
import { decodeJwtClaims } from '../auth/claims'

function getErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof AuthApiError) return err.message
  if (err instanceof Error) return err.message
  return fallback
}

// MyProfilePage is the logged-in user's own profile (/me): hero (avatar, name,
// level/XP, headline stats), tabs (Overview / Rating / Achievements / Account).
// Guests get a limited view with a prompt to register.
export function MyProfilePage() {
  const navigate = useNavigate()
  const { token, isAuthenticated, login } = useAuth()
  const claims = decodeJwtClaims(token)
  const isGuest = claims.isGuest

  const [stats, setStats] = useState<UserStatsDto | null>(null)
  const [me, setMe] = useState<MeResponse | null>(null)
  const [earned, setEarned] = useState<EarnedAchievementDto[]>([])
  const [achievementCatalog, setAchievementCatalog] = useState<AchievementDto[]>([])
  const [ratingEvents, setRatingEvents] = useState<RatingEventDto[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [showEdit, setShowEdit] = useState(false)
  const [showDelete, setShowDelete] = useState(false)
  const [cancelBusy, setCancelBusy] = useState(false)

  useEffect(() => {
    if (!isAuthenticated) {
      navigate('/auth', { replace: true })
    }
  }, [isAuthenticated, navigate])

  useEffect(() => {
    if (!isAuthenticated || isGuest) return
    let cancelled = false
    getMe(token)
      .then((response) => {
        if (cancelled) return
        setMe(response)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setError(getErrorMessage(err, 'Failed to load your profile'))
      })
    return () => {
      cancelled = true
    }
  }, [isAuthenticated, isGuest, token])

  // Registered users have a stats row (zeroed before their first game) and
  // achievements. Guests are blocked server-side, so skip the calls entirely.
  useEffect(() => {
    if (!isAuthenticated || isGuest) return
    let cancelled = false
    Promise.resolve()
      .then(() => {
        if (cancelled) return null
        setIsLoading(true)
        setError(null)
        return getMyStats(token)
      })
      .then((response) => {
        if (cancelled || response === null) return
        setStats(response)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setError(getErrorMessage(err, 'Failed to load your profile'))
      })
      .finally(() => {
        if (cancelled) return
        setIsLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [isAuthenticated, isGuest, token])

  useEffect(() => {
    if (!isAuthenticated || isGuest || !claims.userId) return
    let cancelled = false
    getUserAchievements(token, claims.userId)
      .then((response) => {
        if (cancelled) return
        setEarned(response.earned)
        setAchievementCatalog(response.catalog)
      })
      .catch(() => {
        // Achievements are supplementary; a failure shouldn't block the profile.
      })
    return () => {
      cancelled = true
    }
  }, [isAuthenticated, isGuest, claims.userId, token])

  // Rating history powers the Rating tab; supplementary, so hide on error/empty.
  useEffect(() => {
    if (!isAuthenticated || isGuest || !claims.userId) return
    let cancelled = false
    getRatingHistory(token, claims.userId)
      .then((response) => {
        if (cancelled) return
        setRatingEvents(response.events)
      })
      .catch(() => {
        if (!cancelled) setRatingEvents([])
      })
    return () => {
      cancelled = true
    }
  }, [isAuthenticated, isGuest, claims.userId, token])

  const displayName = stats?.display_name ?? me?.display_name ?? claims.displayName ?? 'Player'
  const avatarUrl = stats?.avatar_url ?? me?.avatar_url ?? claims.avatarUrl
  const username = me?.username ?? null
  const deletionScheduledAt = me?.deletion_scheduled_at ?? null
  const deletionFinalizeDate = deletionScheduledAt
    ? new Date(new Date(deletionScheduledAt).getTime() + 7 * 24 * 60 * 60 * 1000)
    : null

  const handleCancelDeletion = async () => {
    setCancelBusy(true)
    setError(null)
    try {
      await cancelDeletion(token)
      setMe((current) => (current ? { ...current, deletion_scheduled_at: null } : current))
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to cancel account deletion'))
    } finally {
      setCancelBusy(false)
    }
  }

  return (
    <SceneShell
      title="My profile"
      eyebrow="Your account"
      action={
        <div className="flex flex-wrap gap-2">
          <Button variant="ghost" onClick={() => navigate('/lobby')}>Back to lobby</Button>
        </div>
      }
    >
      {error ? (
        <div className="mb-4 rounded-spade-md border border-spade-red/50 bg-spade-red-dark/30 px-4 py-3 text-sm text-spade-cream">
          {error}
        </div>
      ) : null}

      {isGuest ? (
        <ProfileView
          displayName={displayName}
          avatarUrl={avatarUrl}
          stats={null}
          heroMeta="Guest player"
          tabs={[]}
        />
      ) : isLoading && !stats ? (
        <p className="py-8 text-center text-sm text-spade-gray-2">Loading your stats…</p>
      ) : (
        <ProfileView
          displayName={displayName}
          username={username}
          avatarUrl={avatarUrl}
          stats={stats}
          heroActions={
            <Button variant="secondary" onClick={() => setShowEdit(true)}>Edit name</Button>
          }
          tabs={[
            {
              id: 'overview',
              label: 'Overview',
              panel: stats ? (
                <StatCards stats={stats} />
              ) : (
                <p className="py-6 text-center text-sm text-spade-gray-2">No stats yet.</p>
              ),
            },
            {
              id: 'rating',
              label: 'Rating',
              panel: ratingEvents.length > 0 ? (
                <RatingHistory events={ratingEvents} />
              ) : (
                <p className="py-6 text-center text-sm text-spade-gray-2">No rated games yet.</p>
              ),
            },
            {
              id: 'achievements',
              label: 'Achievements',
              panel: (
                <BadgeGrid
                  catalog={achievementCatalog}
                  earned={earned.map((a) => a.achievement_id)}
                  earnedAt={Object.fromEntries(earned.map((a) => [a.achievement_id, a.earned_at]))}
                />
              ),
            },
            {
              id: 'account',
              label: 'Account',
              panel: (
                <AccountPanel
                  me={me}
                  deletionScheduledAt={deletionScheduledAt}
                  deletionFinalizeDate={deletionFinalizeDate}
                  cancelBusy={cancelBusy}
                  onEditName={() => setShowEdit(true)}
                  onDelete={() => setShowDelete(true)}
                  onCancelDeletion={() => void handleCancelDeletion()}
                />
              ),
            },
          ]}
        />
      )}

      {isGuest ? (
        <div className="mt-4 rounded-spade-lg border border-spade-gold/30 bg-spade-gold/10 p-5">
          <h3 className="text-lg font-medium text-spade-cream">Playing as a guest</h3>
          <p className="mt-1 text-sm text-spade-gray-2">
            Register an account to track your stats, earn achievements, and add friends.
          </p>
          <div className="mt-4">
            <Button onClick={() => navigate('/register')}>Create an account</Button>
          </div>
        </div>
      ) : null}

      {showEdit ? (
        <EditNameModal
          currentName={displayName}
          token={token}
          onClose={() => setShowEdit(false)}
          onSaved={(newToken) => {
            login(newToken)
            setShowEdit(false)
            const newName = decodeJwtClaims(newToken).displayName
            if (newName) {
              setStats((current) => (current ? { ...current, display_name: newName } : current))
              setMe((current) => (current ? { ...current, display_name: newName } : current))
            }
          }}
        />
      ) : null}

      {showDelete && me ? (
        <DeleteAccountModal
          token={token}
          hasPassword={me.has_password}
          onClose={() => setShowDelete(false)}
          onScheduled={(scheduledAt) => {
            setMe((current) =>
              current ? { ...current, deletion_scheduled_at: scheduledAt } : current,
            )
            setShowDelete(false)
          }}
        />
      ) : null}
    </SceneShell>
  )
}

function AccountPanel({
  me,
  deletionScheduledAt,
  deletionFinalizeDate,
  cancelBusy,
  onEditName,
  onDelete,
  onCancelDeletion,
}: {
  me: MeResponse | null
  deletionScheduledAt: string | null
  deletionFinalizeDate: Date | null
  cancelBusy: boolean
  onEditName: () => void
  onDelete: () => void
  onCancelDeletion: () => void
}) {
  if (!me) {
    return <p className="py-6 text-center text-sm text-spade-gray-2">Loading account…</p>
  }

  return (
    <div className="grid gap-4">
      <div className="rounded-spade-lg border border-spade-cream/15 bg-spade-gray-4/35 p-4">
        <h3 className="text-sm font-semibold uppercase tracking-wide text-spade-gray-2">Account</h3>
        <div className="mt-2 grid gap-1 text-sm text-spade-cream">
          <p>
            <span className="text-spade-gray-3">Username:</span>{' '}
            <span className="font-mono">@{me.username ?? 'unknown'}</span>
          </p>
          <p>
            <span className="text-spade-gray-3">Joined:</span>{' '}
            {me.created_at ? new Date(me.created_at).toLocaleDateString() : 'Unknown'}
          </p>
          <p className="text-spade-gray-3">Linked providers:</p>
          <div className="flex flex-wrap gap-2">
            {me.providers.length === 0 ? (
              <span className="rounded-spade-pill border border-spade-cream/15 bg-spade-bg/40 px-2.5 py-1 text-xs text-spade-gray-2">
                none
              </span>
            ) : me.providers.map((provider) => (
              <ProviderBadge key={provider.provider} provider={provider.provider} />
            ))}
          </div>
        </div>
        <div className="mt-4">
          <Button variant="secondary" onClick={onEditName}>Edit name</Button>
        </div>
      </div>

      {deletionScheduledAt ? (
        <div className="rounded-spade-lg border border-spade-red/40 bg-spade-red-dark/25 p-4">
          <h3 className="text-sm font-semibold uppercase tracking-wide text-[#ffb4a8]">Deletion scheduled</h3>
          <p className="mt-2 text-sm text-spade-cream">
            Your account is scheduled for permanent deletion on{' '}
            <strong>
              {deletionFinalizeDate
                ? deletionFinalizeDate.toLocaleString(undefined, {
                    dateStyle: 'medium',
                    timeStyle: 'short',
                  })
                : 'the end of the grace period'}
            </strong>
            . Until then you can cancel and keep your account.
          </p>
          <div className="mt-4">
            <Button variant="secondary" disabled={cancelBusy} onClick={onCancelDeletion}>
              {cancelBusy ? 'Cancelling…' : 'Cancel deletion'}
            </Button>
          </div>
        </div>
      ) : (
        <div className="rounded-spade-lg border border-spade-red/35 bg-spade-red-dark/15 p-4">
          <h3 className="text-sm font-semibold uppercase tracking-wide text-[#ffb4a8]">Danger zone</h3>
          <p className="mt-2 text-sm text-spade-gray-2">
            Permanently delete your account and personal data after a 7-day grace period. Historical game seats
            become &quot;Deleted User&quot;.
          </p>
          <div className="mt-4">
            <Button variant="danger" onClick={onDelete}>
              Delete account
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}

function EditNameModal({
  currentName,
  token,
  onClose,
  onSaved,
}: {
  currentName: string
  token: string | null
  onClose: () => void
  onSaved: (newToken: string) => void
}) {
  const [name, setName] = useState(currentName)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const trimmed = name.trim()
  const valid = trimmed.length > 0 && trimmed.length <= 50

  const submit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!valid) {
      setError('Display name must be 1-50 characters')
      return
    }
    setBusy(true)
    setError(null)
    try {
      const res = await updateDisplayName(token, trimmed)
      if (!res.jwt) {
        setError('Update did not return a session token. Please try again.')
        return
      }
      onSaved(res.jwt)
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to update name'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title="Edit display name"
      eyebrow="Your account"
      description="This is the name other players see at the table and on the leaderboard."
      onClose={onClose}
    >
      <form onSubmit={submit} className="grid gap-4">
        <label className="grid gap-1.5 text-xs font-medium uppercase text-spade-gray-2">
          Display name
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            maxLength={50}
            autoFocus
            className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream outline-none focus:border-spade-gold focus:ring-2 focus:ring-spade-gold/20"
          />
        </label>
        {error ? (
          <p role="alert" className="text-xs text-spade-red">{error}</p>
        ) : null}
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button type="button" variant="secondary" onClick={onClose}>Cancel</Button>
          <Button type="submit" disabled={busy || !valid || trimmed === currentName}>
            {busy ? 'Saving…' : 'Save'}
          </Button>
        </div>
      </form>
    </Modal>
  )
}

function DeleteAccountModal({
  token,
  hasPassword,
  onClose,
  onScheduled,
}: {
  token: string | null
  hasPassword: boolean
  onClose: () => void
  onScheduled: (scheduledAt: string) => void
}) {
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const canConfirm = !hasPassword || password.length > 0

  const submit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!canConfirm) return
    setBusy(true)
    setError(null)
    try {
      const res = await deleteAccount(token, hasPassword ? password : undefined)
      onScheduled(res.deletion_scheduled_at)
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to schedule account deletion'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title="Delete account"
      eyebrow="Danger zone"
      tone="danger"
      description="This schedules permanent deletion. You have 7 days to cancel."
      onClose={onClose}
    >
      <form onSubmit={(e) => void submit(e)} className="grid gap-4">
        <ul className="list-disc space-y-1 pl-5 text-sm text-spade-gray-2">
          <li>Personal data is removed after 7 days (account, OAuth links, friends, stats, achievements, sessions).</li>
          <li>Historical game seats are kept for other players&apos; history but labeled &quot;Deleted User&quot;.</li>
          <li>You stay signed in until you leave so you can cancel easily during the grace period.</li>
        </ul>
        {hasPassword ? (
          <label className="grid gap-1.5 text-xs font-medium uppercase text-spade-gray-2">
            Current password
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              autoFocus
              className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream outline-none focus:border-spade-red focus:ring-2 focus:ring-spade-red/20"
            />
          </label>
        ) : (
          <p className="text-sm text-spade-gray-2">
            This account uses OAuth only. Confirm deletion below (no password step-up in this version).
          </p>
        )}
        {error ? (
          <p role="alert" className="text-xs text-spade-red">{error}</p>
        ) : null}
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button type="button" variant="secondary" onClick={onClose}>
            Keep account
          </Button>
          <Button type="submit" variant="danger" disabled={busy || !canConfirm}>
            {busy ? 'Scheduling…' : 'Delete my account'}
          </Button>
        </div>
      </form>
    </Modal>
  )
}
