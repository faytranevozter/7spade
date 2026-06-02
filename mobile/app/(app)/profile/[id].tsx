import { useEffect, useState } from 'react'
import { Text, View } from 'react-native'
import { useLocalSearchParams } from 'expo-router'
import { Avatar } from '../../../src/components/Avatar'
import { BadgeGrid } from '../../../src/components/BadgeGrid'
import { Button } from '../../../src/components/Button'
import { SceneShell } from '../../../src/components/SceneShell'
import { StatCards } from '../../../src/components/StatCards'
import { AppHeader } from '../../../src/components/AppHeader'
import { ApiError } from '../../../src/api/client'
import { getUserStats, type UserStatsDto } from '../../../src/api/stats'
import { getUserAchievements, type AchievementDto, type EarnedAchievementDto } from '../../../src/api/achievements'
import { acceptFriendRequest, getFriends, removeFriend, sendFriendRequest } from '../../../src/api/friends'
import { useAuth } from '../../../src/hooks/useAuth'
import { decodeJwtClaims } from '../../../src/auth/claims'
import { initialsForName } from '../../../src/game/cards'

type FriendshipStatus = 'none' | 'incoming' | 'outgoing' | 'accepted'

function getErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return fallback
}

// Native port of web/src/pages/ProfilePage.tsx. Public player stats + earned
// achievements.
export default function ProfileScreen() {
  const { id } = useLocalSearchParams<{ id: string }>()
  const { token, isAuthenticated } = useAuth()
  const claims = decodeJwtClaims(token)
  const isOwnProfile = Boolean(id && claims.userId && id === claims.userId)
  const [stats, setStats] = useState<UserStatsDto | null>(null)
  const [earned, setEarned] = useState<EarnedAchievementDto[]>([])
  const [achievementCatalog, setAchievementCatalog] = useState<AchievementDto[]>([])
  const [friendship, setFriendship] = useState<FriendshipStatus>('none')
  const [friendBusy, setFriendBusy] = useState(false)
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
    if (!id || !isAuthenticated || claims.isGuest || isOwnProfile) return
    let cancelled = false
    getFriends(token)
      .then((response) => {
        if (cancelled) return
        const match = response.friends.find((friend) => friend.user_id === id)
        setFriendship((match?.status as FriendshipStatus) ?? 'none')
      })
      .catch(() => {
        if (!cancelled) setFriendship('none')
      })
    return () => {
      cancelled = true
    }
  }, [id, isOwnProfile, isAuthenticated, claims.isGuest, token])

  const handleAddFriend = async () => {
    if (!id) return
    setFriendBusy(true)
    try {
      const response = await sendFriendRequest(token, { userId: id })
      setFriendship(response.status === 'accepted' ? 'accepted' : 'outgoing')
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to send friend request'))
    } finally {
      setFriendBusy(false)
    }
  }

  const handleAcceptFriend = async () => {
    if (!id) return
    setFriendBusy(true)
    try {
      await acceptFriendRequest(token, id)
      setFriendship('accepted')
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to accept friend request'))
    } finally {
      setFriendBusy(false)
    }
  }

  const handleRemoveFriend = async () => {
    if (!id) return
    setFriendBusy(true)
    try {
      await removeFriend(token, id)
      setFriendship('none')
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to update friendship'))
    } finally {
      setFriendBusy(false)
    }
  }

  useEffect(() => {
    if (!id) return
    let cancelled = false
    getUserAchievements(token, id)
      .then((response) => {
        if (cancelled) return
        setEarned(response.earned)
        setAchievementCatalog(response.catalog)
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
            {isAuthenticated && !claims.isGuest ? (
              <View className="flex-row flex-wrap gap-2">
                {isOwnProfile ? (
                  <View className="rounded-spade-pill border border-spade-cream/15 px-3 py-2">
                    <Text className="text-xs text-spade-gray-2">This is your profile</Text>
                  </View>
                ) : friendship === 'none' ? (
                  <Button variant="secondary" onPress={handleAddFriend} disabled={friendBusy}>Add friend</Button>
                ) : friendship === 'incoming' ? (
                  <>
                    <Button variant="secondary" onPress={handleAcceptFriend} disabled={friendBusy}>Accept request</Button>
                    <Button variant="ghost" onPress={handleRemoveFriend} disabled={friendBusy}>Decline</Button>
                  </>
                ) : friendship === 'outgoing' ? (
                  <Button variant="ghost" onPress={handleRemoveFriend} disabled={friendBusy}>Cancel request</Button>
                ) : (
                  <Button variant="ghost" onPress={handleRemoveFriend} disabled={friendBusy}>Remove friend</Button>
                )}
              </View>
            ) : null}
            <StatCards stats={stats} />
            <BadgeGrid catalog={achievementCatalog} earned={earned.map((a) => a.achievement_id)} />
          </View>
        ) : null}
      </SceneShell>
    </View>
  )
}
