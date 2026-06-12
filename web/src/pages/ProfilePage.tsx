import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { ApiError } from '../api/client'
import { getMyStats, getRatingHistory, getUserStats, type RatingEventDto, type UserStatsDto } from '../api/stats'
import { getUserAchievements, type AchievementDto, type EarnedAchievementDto } from '../api/achievements'
import { acceptFriendRequest, getFriends, removeFriend, sendFriendRequest } from '../api/friends'
import { Avatar } from '../components/Avatar'
import { BadgeGrid } from '../components/BadgeGrid'
import { Button } from '../components/Button'
import { RatingHistory } from '../components/RatingHistory'
import { SceneShell } from '../components/SceneShell'
import { StatCards } from '../components/StatCards'
import { StatComparison } from '../components/StatComparison'
import { useAuth } from '../hooks/useAuth'
import { decodeJwtClaims } from '../auth/claims'
import { initialsForName } from '../game/cards'

type FriendshipStatus = 'none' | 'incoming' | 'outgoing' | 'accepted'

export function ProfilePage() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const { token, isAuthenticated } = useAuth()
  const claims = decodeJwtClaims(token)
  const isOwnProfile = Boolean(id && claims.userId && id === claims.userId)
  const [stats, setStats] = useState<UserStatsDto | null>(null)
  const [myStats, setMyStats] = useState<UserStatsDto | null>(null)
  const [earned, setEarned] = useState<EarnedAchievementDto[]>([])
  const [achievementCatalog, setAchievementCatalog] = useState<AchievementDto[]>([])
  const [ratingEvents, setRatingEvents] = useState<RatingEventDto[]>([])
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

  // Fetch the viewer's own stats to power the "You vs X" comparison. Gated to
  // authenticated non-guests viewing someone else's profile.
  useEffect(() => {
    const eligible = isAuthenticated && !claims.isGuest && !isOwnProfile
    let cancelled = false
    Promise.resolve()
      .then(() => (eligible && !cancelled ? getMyStats(token) : null))
      .then((response) => {
        if (cancelled) return
        setMyStats(response)
      })
      .catch(() => {
        // Non-fatal: the comparison simply won't render.
        if (!cancelled) setMyStats(null)
      })
    return () => {
      cancelled = true
    }
  }, [isOwnProfile, isAuthenticated, claims.isGuest, token])

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

  // Achievements are supplementary: a failure here shouldn't block the profile.
  useEffect(() => {
    if (!id) return
    let cancelled = false
    getUserAchievements(token, id)
      .then((response) => {
        if (cancelled) return
        setEarned(response.earned)
        setAchievementCatalog(response.catalog)
      })
      .catch(() => {
        // Ignore — the stats above are the primary content.
      })
    return () => {
      cancelled = true
    }
  }, [id, token])

  // Rating history is supplementary too; hide the section on error/empty.
  useEffect(() => {
    if (!id) return
    let cancelled = false
    getRatingHistory(token, id)
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
            {isAuthenticated && !claims.isGuest ? (
              <div className="ml-auto flex flex-wrap gap-2">
                {isOwnProfile ? (
                  <Button variant="secondary" onClick={() => navigate('/me')}>Edit my profile</Button>
                ) : friendship === 'none' ? (
                  <Button variant="secondary" onClick={handleAddFriend} disabled={friendBusy}>Add friend</Button>
                ) : friendship === 'incoming' ? (
                  <>
                    <Button variant="secondary" onClick={handleAcceptFriend} disabled={friendBusy}>Accept request</Button>
                    <Button variant="ghost" onClick={handleRemoveFriend} disabled={friendBusy}>Decline</Button>
                  </>
                ) : friendship === 'outgoing' ? (
                  <Button variant="ghost" onClick={handleRemoveFriend} disabled={friendBusy}>Cancel request</Button>
                ) : (
                  <Button variant="ghost" onClick={handleRemoveFriend} disabled={friendBusy}>Remove friend</Button>
                )}
              </div>
            ) : null}
          </div>
          <StatCards stats={stats} />
          {!isOwnProfile && myStats ? (
            <StatComparison mine={myStats} theirs={stats} opponentName={stats.display_name} />
          ) : null}
          <RatingHistory events={ratingEvents} />
          <BadgeGrid catalog={achievementCatalog} earned={earned.map((a) => a.achievement_id)} earnedAt={Object.fromEntries(earned.map((a) => [a.achievement_id, a.earned_at]))} />
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
