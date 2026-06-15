import { useEffect, useState } from 'react'
import { Text, View } from 'react-native'
import { useLocalSearchParams, useRouter } from 'expo-router'
import { Avatar } from '../../../src/components/Avatar'
import { Badge } from '../../../src/components/Badge'
import { Button } from '../../../src/components/Button'
import { EmotePicker } from '../../../src/components/EmotePicker'
import { GameBoard } from '../../../src/components/GameBoard'
import { ScoreTable } from '../../../src/components/ScoreTable'
import { SceneShell } from '../../../src/components/SceneShell'
import { useAuth } from '../../../src/hooks/useAuth'
import { useSpectatorSocket, type SpectatorPlayer, type SpectatorReaction } from '../../../src/hooks/useSpectatorSocket'
import { initialsForName } from '../../../src/game/cards'
import { emoteGlyph } from '../../../src/game/emotes'
import type { GameResult, Score } from '../../../src/types'

const connectionTone = {
  idle: 'waiting',
  connecting: 'waiting',
  open: 'playing',
  closed: 'danger',
  error: 'danger',
} as const

function resultsToScores(results: GameResult[]): Score[] {
  return results.map((result) => ({
    rank: result.rank,
    player: result.player,
    cardsLeft: 0,
    penalty: result.penalty,
    result: result.winner ? 'Winner' : `Rank ${result.rank}`,
    winner: result.winner,
  }))
}

// Native port of web/src/pages/SpectatorPage.tsx. Read-only live view; no moves.
export default function SpectateScreen() {
  const { id } = useLocalSearchParams<{ id: string }>()
  const router = useRouter()
  const { token } = useAuth()
  const game = useSpectatorSocket(id, token)

  // Tick while a cooldown is pending so the picker re-enables and the countdown
  // updates without an inbound message.
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (game.emoteCooldownUntil <= Date.now()) return undefined
    const timer = setInterval(() => setNow(Date.now()), 250)
    return () => clearInterval(timer)
  }, [game.emoteCooldownUntil])
  const cooldownRemaining = Math.max(0, game.emoteCooldownUntil - now)
  const emoteCoolingDown = cooldownRemaining > 0

  const action = (
    <View className="flex-row flex-wrap gap-2">
      <Badge tone="waiting">Spectating</Badge>
      <Badge tone={game.status === 'open' ? 'playing' : connectionTone[game.status]}>
        {game.status === 'open' ? 'Live' : game.status}
      </Badge>
    </View>
  )

  return (
    <View className="flex-1 bg-spade-bg">
      <SceneShell title="Watching" eyebrow="Spectator" action={action}>
        {game.notFound ? (
          <View className="gap-4 py-8">
            <Text className="text-center text-sm text-spade-gray-2">
              This game isn't available to watch - it may not have started, or has already ended.
            </Text>
            <View className="items-center">
              <Button variant="secondary" onPress={() => router.replace('/(app)/lobby')}>Back to lobby</Button>
            </View>
          </View>
        ) : game.gameOver ? (
          <View className="gap-4">
            <Text className="text-lg font-medium text-spade-cream">Final results</Text>
            <ScoreTable scores={resultsToScores(game.results)} />
            <View className="items-center">
              <Button variant="secondary" onPress={() => router.replace('/(app)/lobby')}>Back to lobby</Button>
            </View>
          </View>
        ) : (
          <View className="gap-4">
            <SpectatorPlayersRow players={game.players} currentTurnName={game.currentTurnName} />
            <View className="w-full">
              <GameBoard rows={game.boardRows} />
              <View className="mt-2 flex-row justify-center">
                <Badge tone="waiting">{game.currentTurnName ? `${game.currentTurnName}'s turn` : 'Waiting...'}</Badge>
              </View>
            </View>
            <Text className="text-center font-mono text-[11px] text-spade-gray-3">Read-only spectator view - you can react, not play.</Text>
            <SpectatorReactions reactions={game.reactions} />
            <View className="flex-row items-center justify-center gap-2">
              {emoteCoolingDown ? (
                <Text className="font-mono text-[10px] text-spade-gray-3">{Math.ceil(cooldownRemaining / 1000)}s</Text>
              ) : null}
              <EmotePicker onSelect={game.sendEmote} disabled={emoteCoolingDown} />
            </View>
          </View>
        )}
      </SceneShell>
    </View>
  )
}

// SpectatorReactions renders incoming spectator emotes as a row of pills,
// styled distinctly (cream) from the player seat emote bubbles.
function SpectatorReactions({ reactions }: { reactions: SpectatorReaction[] }) {
  if (reactions.length === 0) return null
  const visible = reactions.slice(-12)
  return (
    <View
      accessibilityRole="text"
      accessibilityLabel="Spectator reactions"
      className="min-h-[2rem] flex-row flex-wrap items-center justify-center gap-1.5"
    >
      {visible.map((reaction) => {
        const glyph = emoteGlyph(reaction.emote)
        if (!glyph) return null
        const isWord = /[a-zA-Z]/.test(glyph)
        return (
          <View key={reaction.seq} className="rounded-spade-pill border border-spade-cream/20 bg-spade-cream/10 px-2 py-0.5">
            <Text className={isWord ? 'text-xs font-semibold text-spade-cream' : 'text-base text-spade-cream'}>{glyph}</Text>
          </View>
        )
      })}
    </View>
  )
}

function SpectatorPlayersRow({ players, currentTurnName }: { players: SpectatorPlayer[]; currentTurnName: string | null }) {
  if (players.length === 0) return null
  return (
    <View className="w-full flex-row flex-wrap items-end justify-center gap-4">
      {players.map((player) => {
        const isCurrentTurn = player.displayName === currentTurnName
        const ringClass = isCurrentTurn ? 'border-spade-gold' : 'border-spade-cream/10'
        const opacityClass = player.disconnected ? 'opacity-50' : ''
        return (
          <View
            key={player.displayName}
            className={`items-center gap-1.5 rounded-spade-lg border ${ringClass} bg-spade-bg/50 px-3 py-2 ${opacityClass}`}
          >
            <Avatar avatarUrl={player.avatarUrl} initials={initialsForName(player.displayName)} size={36} />
            <Text className="max-w-[80px] text-xs font-medium text-spade-cream" numberOfLines={1}>{player.displayName}</Text>
            <View className="flex-row items-center gap-2">
              <Text className="text-[10px] text-spade-gray-3">H {player.handCount}</Text>
              <Text className="text-[10px] text-spade-gray-3">F {player.faceDownCount}</Text>
            </View>
            {player.disconnected ? <Text className="text-[9px] text-red-400">Offline</Text> : null}
          </View>
        )
      })}
    </View>
  )
}
