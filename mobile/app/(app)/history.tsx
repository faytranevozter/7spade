import { useEffect, useState } from 'react'
import { Text, View } from 'react-native'
import { Badge } from '../../src/components/Badge'
import { Button } from '../../src/components/Button'
import { SceneShell } from '../../src/components/SceneShell'
import { SectionPanel } from '../../src/components/SectionPanel'
import { StatCards } from '../../src/components/StatCards'
import { ApiError } from '../../src/api/client'
import { getHistory, type HistoryGameDto } from '../../src/api/history'
import { getMyStats, type UserStatsDto } from '../../src/api/stats'
import { useAuth } from '../../src/hooks/useAuth'

const perPage = 5

function getErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return fallback
}

function formatDate(value: string): string {
  return new Date(value).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

// Native port of web/src/pages/HistoryPage.tsx. Lifetime stats panel + paginated
// completed-games list. Guests are blocked server-side; this screen is only
// reachable for registered users.
export default function HistoryScreen() {
  const { token } = useAuth()
  const [page, setPage] = useState(1)
  const [games, setGames] = useState<HistoryGameDto[]>([])
  const [total, setTotal] = useState(0)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [stats, setStats] = useState<UserStatsDto | null>(null)
  const totalPages = Math.max(1, Math.ceil(total / perPage))

  useEffect(() => {
    let cancelled = false
    getMyStats(token)
      .then((response) => {
        if (!cancelled) setStats(response)
      })
      .catch(() => {})
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
        return getHistory(token, page, perPage)
      })
      .then((response) => {
        if (cancelled || response === null) return
        setGames(response.games)
        setTotal(response.total)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setError(getErrorMessage(err, 'Failed to load game history'))
      })
      .finally(() => {
        if (cancelled) return
        setIsLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [page, token])

  return (
    <View className="flex-1 bg-spade-bg">
      <SceneShell title="Game history" eyebrow="Your results" action={<Badge tone="waiting">{`Page ${page}`}</Badge>}>
        {error ? (
          <View className="mb-4 rounded-spade-md border border-spade-red/50 bg-spade-red-dark/30 px-4 py-3">
            <Text className="text-sm text-spade-cream">{error}</Text>
          </View>
        ) : null}

        {stats ? (
          <View className="mb-4">
            <SectionPanel title="Your stats" eyebrow="Lifetime totals">
              <StatCards stats={stats} />
              {!stats.qualified ? (
                <Text className="mt-3 font-mono text-xs text-spade-gray-3">Play more games to join the leaderboard.</Text>
              ) : null}
            </SectionPanel>
          </View>
        ) : null}

        <View className="overflow-hidden rounded-spade-lg border border-spade-cream/12 bg-[#2b302d]">
          <View className="flex-row border-b border-spade-cream/10 bg-spade-cream/5 px-3 py-2">
            <Text className="flex-1 font-mono text-[10px] uppercase text-spade-gray-3">Started</Text>
            <Text className="w-20 font-mono text-[10px] uppercase text-spade-gray-3">Result</Text>
            <Text className="w-14 text-right font-mono text-[10px] uppercase text-spade-gray-3">Penalty</Text>
            <Text className="w-14 text-right font-mono text-[10px] uppercase text-spade-gray-3">Rating</Text>
          </View>
          {games.map((game) => (
            <View key={game.game_id} className="flex-row items-center border-t border-spade-cream/10 px-3 py-3">
              <Text className="flex-1 text-xs text-spade-gray-2">{formatDate(game.started_at)}</Text>
              <Text className="w-20 text-sm text-spade-cream">{game.is_winner ? 'Winner' : `Rank ${game.rank}`}</Text>
              <Text className="w-14 text-right font-mono text-sm text-spade-gold-light">{game.penalty_points}</Text>
              <Text className={`w-14 text-right font-mono text-sm ${game.rating_delta != null ? (game.rating_delta > 0 ? 'text-green-400' : game.rating_delta < 0 ? 'text-red-400' : 'text-spade-gray-2') : 'text-spade-gray-2'}`}>
                {game.rating_delta != null ? `${game.rating_delta >= 0 ? '+' : ''}${game.rating_delta}` : '—'}
              </Text>
            </View>
          ))}
          {!isLoading && games.length === 0 ? (
            <View className="border-t border-spade-cream/10 px-4 py-8">
              <Text className="text-center text-sm text-spade-gray-2">No completed games yet.</Text>
            </View>
          ) : null}
        </View>

        <View className="mt-4 flex-row flex-wrap items-center justify-between gap-3">
          <Text className="font-mono text-xs text-spade-gray-3">
            {isLoading ? 'Loading games...' : `${total} games - page ${page} of ${totalPages}`}
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
