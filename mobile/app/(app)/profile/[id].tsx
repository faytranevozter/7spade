import { useEffect, useState } from 'react'
import { Text, View } from 'react-native'
import { useLocalSearchParams } from 'expo-router'
import { Avatar } from '../../../src/components/Avatar'
import { BadgeGrid } from '../../../src/components/BadgeGrid'
import { SceneShell } from '../../../src/components/SceneShell'
import { StatCards } from '../../../src/components/StatCards'
import { AppHeader } from '../../../src/components/AppHeader'
import { ApiError } from '../../../src/api/client'
import { getUserStats, type UserStatsDto } from '../../../src/api/stats'
import { getUserAchievements, type EarnedAchievementDto } from '../../../src/api/achievements'
import { useAuth } from '../../../src/hooks/useAuth'
import { initialsForName } from '../../../src/game/cards'

function getErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return fallback
}

// Native port of web/src/pages/ProfilePage.tsx. Public player stats + earned
// achievements.
export default function ProfileScreen() {
  const { id } = useLocalSearchParams<{ id: string }>()
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

  useEffect(() => {
    if (!id) return
    let cancelled = false
    getUserAchievements(token, id)
      .then((response) => {
        if (!cancelled) setEarned(response.earned)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [id, token])

  const title = stats ? stats.display_name : 'Player profile'

  return (
    <View className="flex-1 bg-spade-bg">
      <AppHeader />
      <SceneShell title={title} eyebrow="Player stats">
        {error ? (
          <View className="mb-4 rounded-spade-md border border-spade-red/50 bg-spade-red-dark/30 px-4 py-3">
            <Text className="text-sm text-spade-cream">{error}</Text>
          </View>
        ) : null}
        {notFound ? (
          <Text className="py-8 text-center text-sm text-spade-gray-2">
            Player not found - they may not have played a recorded game yet.
          </Text>
        ) : isLoading ? (
          <Text className="py-8 text-center text-sm text-spade-gray-2">Loading player...</Text>
        ) : stats ? (
          <View className="gap-4">
            <View className="flex-row items-center gap-3">
              <Avatar avatarUrl={stats.avatar_url} initials={initialsForName(stats.display_name)} alt={stats.display_name} size={56} />
              <Text className="text-lg font-medium text-spade-cream">{stats.display_name}</Text>
            </View>
            <StatCards stats={stats} />
            <BadgeGrid earned={earned.map((a) => a.achievement_id)} />
          </View>
        ) : null}
      </SceneShell>
    </View>
  )
}
