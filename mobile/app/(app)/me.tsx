import { useEffect, useState } from 'react'
import { Text, TextInput, View } from 'react-native'
import { useRouter } from 'expo-router'
import { Avatar } from '../../src/components/Avatar'
import { BadgeGrid } from '../../src/components/BadgeGrid'
import { Button } from '../../src/components/Button'
import { Modal } from '../../src/components/Modal'
import { SceneShell } from '../../src/components/SceneShell'
import { StatCards } from '../../src/components/StatCards'
import { AppHeader } from '../../src/components/AppHeader'
import { ApiError } from '../../src/api/client'
import { AuthApiError, deleteLogout, getMe, updateDisplayName, type MeResponse } from '../../src/api/auth'
import { getMyStats, type UserStatsDto } from '../../src/api/stats'
import { getUserAchievements, type AchievementDto, type EarnedAchievementDto } from '../../src/api/achievements'
import { useAuth } from '../../src/hooks/useAuth'
import { useSound } from '../../src/hooks/useSound'
import { decodeJwtClaims } from '../../src/auth/claims'
import { initialsForName } from '../../src/game/cards'

function getErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof AuthApiError) return err.message
  if (err instanceof Error) return err.message
  return fallback
}

// "My profile" screen (/(app)/me): the logged-in user's own avatar, display
// name (editable), lifetime stats, and achievements. Guests get a limited view
// with a register prompt (they have no DB row and are blocked from /stats,
// /history, /friends server-side). Other players' profiles live at profile/[id].
export default function MyProfileScreen() {
  const router = useRouter()
  const { token, refreshToken, login, logout } = useAuth()
  const { muted, toggleMuted } = useSound()
  const claims = decodeJwtClaims(token)
  const isGuest = claims.isGuest

  const handleSignOut = () => {
    const rt = refreshToken
    logout()
    // Best-effort server-side revoke; local logout above is what matters. The
    // root auth gate redirects to the auth screen once the session clears.
    void deleteLogout(rt).catch(() => {})
  }

  const [stats, setStats] = useState<UserStatsDto | null>(null)
  const [me, setMe] = useState<MeResponse | null>(null)
  const [earned, setEarned] = useState<EarnedAchievementDto[]>([])
  const [achievementCatalog, setAchievementCatalog] = useState<AchievementDto[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [showEdit, setShowEdit] = useState(false)

  useEffect(() => {
    if (isGuest) return
    let cancelled = false
    getMe(token)
      .then((response) => {
        if (!cancelled) setMe(response)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setError(getErrorMessage(err, 'Failed to load your profile'))
      })
    return () => {
      cancelled = true
    }
  }, [isGuest, token])

  useEffect(() => {
    if (isGuest) return
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
  }, [isGuest, token])

  useEffect(() => {
    if (isGuest || !claims.userId) return
    let cancelled = false
    getUserAchievements(token, claims.userId)
      .then((response) => {
        if (cancelled) return
        setEarned(response.earned)
        setAchievementCatalog(response.catalog)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [isGuest, claims.userId, token])

  const displayName = stats?.display_name ?? me?.display_name ?? claims.displayName ?? 'Player'

  return (
    <View className="flex-1 bg-spade-bg">
      <AppHeader />
      <SceneShell title="My profile" eyebrow="Your account">
        {error ? (
          <View className="mb-4 rounded-spade-md border border-spade-red/50 bg-spade-red-dark/30 px-4 py-3">
            <Text className="text-sm text-spade-cream">{error}</Text>
          </View>
        ) : null}

        <View className="gap-4">
          <View className="flex-row items-center gap-3">
            <Avatar avatarUrl={claims.avatarUrl} initials={initialsForName(displayName)} alt={displayName} size={56} />
            <View className="flex-1">
              <Text className="text-lg font-medium text-spade-cream">{displayName}</Text>
              <Text className="font-mono text-xs text-spade-gray-3">{isGuest ? 'Guest player' : 'Registered player'}</Text>
            </View>
            {!isGuest ? (
              <Button variant="secondary" onPress={() => setShowEdit(true)}>Edit name</Button>
            ) : null}
          </View>

          {isGuest ? (
            <View className="rounded-spade-lg border border-spade-gold/30 bg-spade-gold/10 p-5">
              <Text className="text-lg font-medium text-spade-cream">Playing as a guest</Text>
              <Text className="mt-1 text-sm text-spade-gray-2">
                Register an account to track your stats, earn achievements, and add friends.
              </Text>
            </View>
          ) : (
            <>
              {/* Account details (username / joined / linked providers). */}
              {me ? (
                <View className="rounded-spade-lg border border-spade-cream/15 bg-spade-gray-4/35 p-4">
                  <Text className="text-xs font-semibold uppercase tracking-wider text-spade-gray-2">Account</Text>
                  <Text className="mt-2 text-sm text-spade-cream">
                    <Text className="text-spade-gray-3">Username: </Text>
                    <Text className="font-mono">@{me.username ?? 'unknown'}</Text>
                  </Text>
                  <Text className="mt-1 text-sm text-spade-cream">
                    <Text className="text-spade-gray-3">Joined: </Text>
                    {me.created_at ? new Date(me.created_at).toLocaleDateString() : 'Unknown'}
                  </Text>
                  <Text className="mt-2 text-xs uppercase tracking-wider text-spade-gray-3">Linked providers</Text>
                  <View className="mt-1 flex-row flex-wrap gap-2">
                    {me.providers.length === 0 ? (
                      <View className="rounded-spade-pill border border-spade-cream/15 px-2 py-1">
                        <Text className="text-xs text-spade-gray-2">none</Text>
                      </View>
                    ) : me.providers.map((provider) => (
                      <View key={provider.provider} className="rounded-spade-pill border border-spade-cream/15 px-2 py-1">
                        <Text className="text-xs uppercase tracking-wider text-spade-cream">{provider.provider}</Text>
                      </View>
                    ))}
                  </View>
                </View>
              ) : null}

              {/* Lifetime stats + achievements. These stack below the account
                  block — they are not alternatives to it. */}
              {isLoading && !stats ? (
                <Text className="py-8 text-center text-sm text-spade-gray-2">Loading your stats...</Text>
              ) : stats ? (
                <View className="gap-4">
                  <StatCards stats={stats} />
                  <BadgeGrid catalog={achievementCatalog} earned={earned.map((a) => a.achievement_id)} />
                </View>
              ) : null}
            </>
          )}

          {/* Account actions live here (moved off the header, which overflowed
              the phone width). Available to guests and registered users alike. */}
          <View className="mt-2 gap-2 border-t border-spade-cream/10 pt-4">
            {!isGuest ? (
              <Button variant="ghost" onPress={() => router.push('/(app)/friends')}>Manage friends</Button>
            ) : null}
            <Button variant="secondary" onPress={toggleMuted}>
              {muted ? 'Unmute sound' : 'Mute sound'}
            </Button>
            <Button variant="danger" onPress={handleSignOut}>Sign out</Button>
          </View>
        </View>
      </SceneShell>

      {showEdit ? (
        <EditNameModal
          currentName={displayName}
          token={token}
          onClose={() => setShowEdit(false)}
          onSaved={(newToken) => {
            login(newToken, refreshToken)
            setShowEdit(false)
            // Reflect the new name immediately across both data sources so the
            // displayed name (stats -> me -> claims fallback) updates regardless
            // of which is populated.
            const newName = decodeJwtClaims(newToken).displayName
            if (newName) {
              setStats((current) => (current ? { ...current, display_name: newName } : current))
              setMe((current) => (current ? { ...current, display_name: newName } : current))
            }
          }}
        />
      ) : null}
    </View>
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

  const submit = async () => {
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
      footer={
        <>
          <Button onPress={submit} disabled={busy || !valid || trimmed === currentName}>
            {busy ? 'Saving...' : 'Save'}
          </Button>
          <Button variant="secondary" onPress={onClose}>Cancel</Button>
        </>
      }
    >
      <View className="gap-2">
        <Text className="text-xs font-medium uppercase text-spade-gray-2">Display name</Text>
        <TextInput
          value={name}
          onChangeText={setName}
          maxLength={50}
          autoFocus
          className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream"
        />
        {error ? <Text className="text-xs text-spade-red">{error}</Text> : null}
      </View>
    </Modal>
  )
}
