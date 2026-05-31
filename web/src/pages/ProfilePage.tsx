import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { ApiError } from '../api/client'
import { getUserStats, type UserStatsDto } from '../api/stats'
import { getUserAchievements, type EarnedAchievementDto } from '../api/achievements'
import { Avatar } from '../components/Avatar'
import { BadgeGrid } from '../components/BadgeGrid'
import { Button } from '../components/Button'
import { SceneShell } from '../components/SceneShell'
import { StatCards } from '../components/StatCards'
import { useAuth } from '../hooks/useAuth'
import { initialsForName } from '../game/cards'

export function ProfilePage() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const { token } = useAuth()
  const [stats, setStats] = useState<UserStatsDto | null>(null)
  const [earned, setEarned] = useState<EarnedAchievementDto[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notFound, setNotFound] = useState(false)

  useEffect(() => {
    if (!id) return
    let cancelled = false
    Promise.resolve()
      .then(() => {
        if (cancelled) return null
        setIsLoading(true)
        setError(null)
        setNotFound(false)
        return getUserStats(token, id)
      })
      .then((response) => {
        if (cancelled || response === null) return
        setStats(response)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        if (err instanceof ApiError && err.statusCode === 404) {
          setNotFound(true)
          return
        }
        setError(getErrorMessage(err, 'Failed to load player'))
      })
      .finally(() => {
        if (cancelled) return
        setIsLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [id, token])

  // Achievements are supplementary: a failure here shouldn't block the profile.
  useEffect(() => {
    if (!id) return
    let cancelled = false
    getUserAchievements(token, id)
      .then((response) => {
        if (cancelled) return
        setEarned(response.earned)
      })
      .catch(() => {
        // Ignore — the stats above are the primary content.
      })
    return () => {
      cancelled = true
    }
  }, [id, token])

  const title = stats ? stats.display_name : 'Player profile'

  return (
    <SceneShell
      title={title}
      eyebrow="Player stats"
      action={<Button variant="ghost" onClick={() => navigate('/leaderboard')}>Back to leaderboard</Button>}
    >
      {error ? (
        <div className="mb-4 rounded-spade-md border border-spade-red/50 bg-spade-red-dark/30 px-4 py-3 text-sm text-spade-cream">
          {error}
        </div>
      ) : null}
      {notFound ? (
        <p className="py-8 text-center text-sm text-spade-gray-2">
          Player not found — they may not have played a recorded game yet.
        </p>
      ) : isLoading ? (
        <p className="py-8 text-center text-sm text-spade-gray-2">Loading player...</p>
      ) : stats ? (
        <div className="grid gap-4">
          <div className="flex items-center gap-3">
            <Avatar
              avatarUrl={stats.avatar_url}
              initials={initialsForName(stats.display_name)}
              alt={stats.display_name}
              sizeClass="size-14"
              className="text-lg"
            />
            <p className="text-lg font-medium text-spade-cream">{stats.display_name}</p>
          </div>
          <StatCards stats={stats} />
          <BadgeGrid earned={earned.map((a) => a.achievement_id)} earnedAt={Object.fromEntries(earned.map((a) => [a.achievement_id, a.earned_at]))} />
        </div>
      ) : null}
    </SceneShell>
  )
}

function getErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return fallback
}
