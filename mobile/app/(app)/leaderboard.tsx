import { useEffect, useState } from 'react'
import { Pressable, ScrollView, Text, View } from 'react-native'
import { useRouter } from 'expo-router'
import { Avatar } from '../../src/components/Avatar'
import { Badge } from '../../src/components/Badge'
import { Button } from '../../src/components/Button'
import { SceneShell } from '../../src/components/SceneShell'
import { ApiError } from '../../src/api/client'
import { getLeaderboard, getSeasons, type LeaderboardEntryDto, type SeasonDto } from '../../src/api/stats'
import { useAuth } from '../../src/hooks/useAuth'
import { initialsForName } from '../../src/game/cards'

const perPage = 10

// The all-time leaderboard is the default scope; '' maps to no season param.
const ALL_TIME = ''

function getErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return fallback
}

function formatPercent(value: number): string {
  return `${(value * 100).toFixed(1)}%`
}

// Native port of web/src/pages/LeaderboardPage.tsx. Season-scoped rankings (a
// chip row to switch season), tap a row to open that player's profile.
export default function LeaderboardScreen() {
  const router = useRouter()
  const { token } = useAuth()
  const [page, setPage] = useState(1)
  const [season, setSeason] = useState<string>(ALL_TIME)
  const [seasons, setSeasons] = useState<SeasonDto[]>([])
  const [entries, setEntries] = useState<LeaderboardEntryDto[]>([])
  const [total, setTotal] = useState(0)
  const [minGames, setMinGames] = useState(0)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const totalPages = Math.max(1, Math.ceil(total / perPage))

  // Load the season list once so the chip row can offer all-time + each season.
  useEffect(() => {
    let cancelled = false
    getSeasons(token)
      .then((response) => {
        if (!cancelled) setSeasons(response.seasons)
      })
      .catch(() => {
        if (!cancelled) setSeasons([])
      })
    return () => {
      cancelled = true
    }
  }, [token])

  useEffect(() => {
    let cancelled = false
    Promise.resolve()
      .then(() => {
        if (cancelled) return null
        setIsLoading(true)
        setError(null)
        return getLeaderboard(token, page, perPage, season)
      })
      .then((response) => {
        if (cancelled || response === null) return
        setEntries(response.entries)
        setTotal(response.total)
        setMinGames(response.min_games)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setError(getErrorMessage(err, 'Failed to load leaderboard'))
      })
      .finally(() => {
        if (cancelled) return
        setIsLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [page, season, token])

  function selectSeason(next: string) {
    setSeason(next)
    setPage(1)
  }

  const eyebrow = season === ALL_TIME ? 'All-time rankings' : (seasons.find((s) => s.id === season)?.label ?? `Season ${season}`)

  const chips: { id: string; label: string }[] = [
    { id: ALL_TIME, label: 'All-time' },
    ...seasons.map((s) => ({ id: s.id, label: s.active ? `${s.label} (current)` : s.label })),
  ]

  return (
    <View className="flex-1 bg-spade-bg">
      <SceneShell title="Leaderboard" eyebrow={eyebrow} action={<Badge tone="winner">{`Page ${page}`}</Badge>}>
        {error ? (
          <View className="mb-4 rounded-spade-md border border-spade-red/50 bg-spade-red-dark/30 px-4 py-3">
            <Text className="text-sm text-spade-cream">{error}</Text>
          </View>
        ) : null}

        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          className="mb-3"
          contentContainerClassName="flex-row gap-2"
        >
          {chips.map((chip) => {
            const isActive = chip.id === season
            return (
              <Pressable
                key={chip.id || 'all-time'}
                onPress={() => selectSeason(chip.id)}
                accessibilityRole="button"
                accessibilityState={{ selected: isActive }}
                className={`rounded-spade-md border px-3 py-1.5 ${
                  isActive ? 'border-spade-gold bg-spade-gold/15' : 'border-spade-cream/15 bg-spade-bg'
                }`}
              >
                <Text className={`font-mono text-xs ${isActive ? 'text-spade-gold-light' : 'text-spade-gray-3'}`}>
                  {chip.label}
                </Text>
              </Pressable>
            )
          })}
        </ScrollView>

        <View className="overflow-hidden rounded-spade-lg border border-spade-cream/12 bg-[#2b302d]">
          <View className="flex-row border-b border-spade-cream/10 bg-spade-cream/5 px-3 py-2">
            <Text className="w-8 font-mono text-[10px] uppercase text-spade-gray-3">#</Text>
            <Text className="flex-1 font-mono text-[10px] uppercase text-spade-gray-3">Player</Text>
            <Text className="w-14 text-right font-mono text-[10px] uppercase text-spade-gray-3">Rating</Text>
            <Text className="w-14 text-right font-mono text-[10px] uppercase text-spade-gray-3">Win %</Text>
          </View>
          {entries.map((entry) => (
            <Pressable
              key={entry.user_id}
              onPress={() => router.push(`/(app)/profile/${entry.user_id}`)}
              className="flex-row items-center border-t border-spade-cream/10 px-3 py-3 active:bg-spade-cream/5"
            >
              <Text className="w-8 font-mono text-spade-gold-light">{entry.rank}</Text>
              <View className="flex-1 flex-row items-center gap-2">
                <Avatar avatarUrl={entry.avatar_url} initials={initialsForName(entry.display_name)} size={28} />
                <Text className="flex-1 text-sm text-spade-cream" numberOfLines={1}>{entry.display_name}</Text>
              </View>
              <Text className="w-14 text-right font-mono text-sm text-spade-gold-light">{entry.rating}</Text>
              <Text className="w-14 text-right font-mono text-sm text-spade-cream">{formatPercent(entry.win_rate)}</Text>
            </Pressable>
          ))}
          {!isLoading && entries.length === 0 ? (
            <View className="border-t border-spade-cream/10 px-4 py-8">
              <Text className="text-center text-sm text-spade-gray-2">No ranked players yet - play a few games to appear.</Text>
            </View>
          ) : null}
        </View>

        <View className="mt-4 flex-row flex-wrap items-center justify-between gap-3">
          <Text className="font-mono text-xs text-spade-gray-3">
            {isLoading ? 'Loading leaderboard...' : `${total} ranked - min ${minGames} games to qualify`}
          </Text>
          <View className="flex-row gap-2">
            <Button variant="secondary" disabled={page <= 1} onPress={() => setPage((c) => Math.max(1, c - 1))}>Previous</Button>
            <Button variant="secondary" disabled={page >= totalPages} onPress={() => setPage((c) => Math.min(totalPages, c + 1))}>Next</Button>
          </View>
        </View>
      </SceneShell>
    </View>
  )
}
