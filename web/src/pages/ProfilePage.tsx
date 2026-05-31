import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { ApiError } from '../api/client'
import { getUserStats, type UserStatsDto } from '../api/stats'
import { Button } from '../components/Button'
import { SceneShell } from '../components/SceneShell'
import { StatCards } from '../components/StatCards'
import { useAuth } from '../hooks/useAuth'

export function ProfilePage() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const { token } = useAuth()
  const [stats, setStats] = useState<UserStatsDto | null>(null)
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
        <StatCards stats={stats} />
      ) : null}
    </SceneShell>
  )
}

function getErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return fallback
}
