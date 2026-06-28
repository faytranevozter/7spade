import { useEffect, useMemo, useRef, useState } from 'react'
import { Pressable, ScrollView, Text, View } from 'react-native'
import { SafeAreaView } from 'react-native-safe-area-context'
import { useLocalSearchParams, useRouter } from 'expo-router'
import { Badge } from '../../../src/components/Badge'
import { Button } from '../../../src/components/Button'
import { Avatar } from '../../../src/components/Avatar'
import { CardFace } from '../../../src/components/CardFace'
import { EmoteBubble } from '../../../src/components/EmoteBubble'
import { EmotePicker } from '../../../src/components/EmotePicker'
import { GameBoard } from '../../../src/components/GameBoard'
import { Modal } from '../../../src/components/Modal'
import { ScoreTable } from '../../../src/components/ScoreTable'
import { SceneShell } from '../../../src/components/SceneShell'
import { ToastStack } from '../../../src/components/ToastStack'
import { ApiError } from '../../../src/api/client'
import { getRoom } from '../../../src/api/lobby'
import { useAuth } from '../../../src/hooks/useAuth'
import { useGameSocket, type ActiveEmote, type GameSocketState, type PlayerSpectatorReaction } from '../../../src/hooks/useGameSocket'
import { useSound } from '../../../src/hooks/useSound'
import { emoteGlyph } from '../../../src/game/emotes'
import type { Card, GameResult, Player } from '../../../src/types'

const connectionTone = {
  idle: 'waiting',
  connecting: 'waiting',
  open: 'playing',
  closed: 'danger',
  error: 'danger',
} as const

export default function GameScreen() {
  const { id } = useLocalSearchParams<{ id: string }>()
  const roomId = id
  const router = useRouter()
  const { token } = useAuth()
  const game = useGameSocket(roomId, token)
  const { play: playSound, unlock: unlockSound } = useSound()
  const hasValidMoves = game.hand.some((card) => card.playable)
  const faceDownMode = game.isMyTurn && game.hand.length > 0 && !hasValidMoves
  const [closePrompt, setClosePrompt] = useState<Card | null>(null)
  const [selectedFaceDown, setSelectedFaceDown] = useState<Card | null>(null)
  const warnedTurnRef = useRef<string | null>(null)

  // TODO: Orientation lock disabled due to modal crash issues
  // React Native modals conflict with ScreenOrientation.lockAsync
  // Re-enable after finding a modal-safe orientation lock solution

  // Verify the room exists / is still playable before staying. A 404 sends the
  // player back to the lobby; a 'finished' room sends them to history.
  useEffect(() => {
    if (!roomId || !token) return
    let cancelled = false
    getRoom(token, roomId)
      .then((room) => {
        if (cancelled) return
        if (room.status === 'finished') {
          router.replace('/(app)/history')
        }
      })
      .catch((err: unknown) => {
        if (cancelled) return
        if (err instanceof ApiError && err.statusCode === 404) {
          router.replace('/(app)/lobby')
        }
      })
    return () => {
      cancelled = true
    }
  }, [roomId, token, router])

  // Clear a stale face-down selection when leaving face-down mode.
  const [wasFaceDownMode, setWasFaceDownMode] = useState(faceDownMode)
  if (faceDownMode !== wasFaceDownMode) {
    setWasFaceDownMode(faceDownMode)
    if (!faceDownMode) {
      setSelectedFaceDown(null)
    }
  }

  const activeFaceDown = faceDownMode && selectedFaceDown
    && game.hand.some((card) => card.rank === selectedFaceDown.rank && card.suit === selectedFaceDown.suit)
    ? selectedFaceDown
    : null

  const visibleHand = useMemo(() => game.hand.map((card) => ({
    ...card,
    dimmed: game.isMyTurn && hasValidMoves && !card.playable,
    selected: faceDownMode
      ? activeFaceDown?.rank === card.rank && activeFaceDown?.suit === card.suit
      : card.selected,
  })), [game.hand, game.isMyTurn, hasValidMoves, faceDownMode, activeFaceDown])

  const playCard = (card: Card) => {
    if (!game.isMyTurn || !card.playable) return
    unlockSound()
    if (card.aceClose) {
      const { canLow, canHigh } = card.aceClose
      if (canLow && canHigh) {
        setClosePrompt(card)
        return
      }
      game.sendPlayCard({ rank: card.rank, suit: card.suit, playable: card.playable }, canLow ? 'low' : 'high')
      return
    }
    game.sendPlayCard({ rank: card.rank, suit: card.suit, playable: card.playable })
  }

  const confirmClose = (method: 'low' | 'high') => {
    if (!closePrompt) return
    game.sendPlayCard({ rank: closePrompt.rank, suit: closePrompt.suit, playable: true }, method)
    setClosePrompt(null)
  }

  const confirmFaceDown = () => {
    if (!activeFaceDown) return
    unlockSound()
    game.sendFaceDown({ rank: activeFaceDown.rank, suit: activeFaceDown.suit })
    setSelectedFaceDown(null)
  }

  const turnLabel = game.currentTurnName ? `${game.currentTurnName}'s turn` : 'Waiting...'
  const turnClock = useTurnClock(game.turnEndsAt)

  // Fire the timer-warning cue once when the local turn drops to ~5s.
  useEffect(() => {
    if (!game.isMyTurn || !game.turnEndsAt) return
    const secondsLeft = Math.max(0, Math.ceil((Date.parse(game.turnEndsAt) - Date.now()) / 1000))
    if (secondsLeft <= 5 && secondsLeft > 0 && warnedTurnRef.current !== game.turnEndsAt) {
      warnedTurnRef.current = game.turnEndsAt
      playSound('timer_warning')
    }
  }, [game.isMyTurn, game.turnEndsAt, turnClock, playSound])

  if (game.gameOver) {
    return <GameOverPanel roomId={roomId} game={game} />
  }

  const statusLabel = game.status === 'open' ? 'Connected' : game.status

  return (
    <SafeAreaView edges={['top', 'bottom']} className="flex-1 bg-spade-bg">
      {/* Landscape 2-row layout: top = game area (opponents + board + timer), bottom = hand strip */}
      
      {/* Top game area - horizontal layout */}
      <View className="flex-1 flex-row gap-2 px-2 py-2">
        {/* Left: Opponents column */}
        <View className="w-16 justify-center gap-2">
          <OpponentsColumn players={game.players} currentTurnName={game.currentTurnName} emotes={game.emotes} />
        </View>

        {/* Center: Board */}
        <View className="flex-1 justify-center">
          <GameBoard rows={game.boardRows} />
        </View>

        {/* Right: Timer & turn indicator */}
        <View className="w-20 items-center justify-center gap-2">
          {turnClock ? (
            <View className="items-center gap-1">
              <Text className="font-mono text-lg font-semibold text-spade-gold-light">
                {turnClock.label}
              </Text>
              <View className="h-20 w-2 overflow-hidden rounded-full bg-spade-bg/70">
                <View 
                  className="w-full rounded-full bg-spade-gold-light" 
                  style={{ height: `${turnClock.percentRemaining}%`, position: 'absolute', bottom: 0 }} 
                />
              </View>
            </View>
          ) : null}
          <Badge tone={game.isMyTurn ? 'playing' : 'waiting'}>
            {game.isMyTurn ? 'YOU' : 'WAIT'}
          </Badge>
          {game.practiceMode ? <Badge tone="winner">Practice</Badge> : null}
          {game.status !== 'open' ? (
            <Badge tone={connectionTone[game.status] ?? 'danger'}>{statusLabel}</Badge>
          ) : null}
        </View>
      </View>

      {/* Bottom: Hand strip */}
      <View className="border-t border-spade-cream/10 bg-spade-bg/90">
        <PlayerHandStrip
          cards={visibleHand}
          interactive={game.isMyTurn && hasValidMoves}
          onCardPress={playCard}
          isMyTurn={game.isMyTurn}
          faceDownMode={faceDownMode}
          onSelectFaceDown={setSelectedFaceDown}
          onConfirmFaceDown={confirmFaceDown}
          hasFaceDownSelection={activeFaceDown !== null}
        />
      </View>

      <SpectatorReactionsOverlay reactions={game.spectatorReactions} />

      <View className="absolute bottom-2 right-2">
        <EmotePicker onSelect={game.sendEmote} />
      </View>

      <View className="absolute left-4 right-4 top-2">
        <ToastStack toasts={game.toasts} />
      </View>

      {game.status === 'error' || game.status === 'closed' ? (
        <View className="absolute right-2 top-2">
          <Button variant="secondary" onPress={game.reconnect}>Reconnect</Button>
        </View>
      ) : null}

      {closePrompt ? (
        <Modal
          title="Close the suit"
          eyebrow={`Ace of ${closePrompt.suit}`}
          description="This Ace can close the suit at either end. Your choice locks the closing method for every suit this round."
          onClose={() => setClosePrompt(null)}
          footer={
            <>
              <Button onPress={() => confirmClose('low')}>Close low (Ace = 1)</Button>
              <Button variant="secondary" onPress={() => confirmClose('high')}>Close high (Ace = 14)</Button>
              <Button variant="ghost" onPress={() => setClosePrompt(null)}>Cancel</Button>
            </>
          }
        />
      ) : null}
    </SafeAreaView>
  )
}

// SpectatorReactionsOverlay shows spectator emotes to seated players, throttled
// by useGameSocket (first few individual, rest aggregated). Floats above the
// emote picker so it reads as crowd reaction, separate from seat emote bubbles.
function SpectatorReactionsOverlay({ reactions }: { reactions: PlayerSpectatorReaction[] }) {
  if (reactions.length === 0) return null
  return (
    <View
      accessibilityRole="text"
      accessibilityLabel="Spectator reactions"
      className="absolute bottom-24 left-4 right-4 flex-row flex-wrap items-center justify-center gap-1.5"
    >
      {reactions.map((reaction) => {
        const glyph = emoteGlyph(reaction.emote)
        if (!glyph) return null
        const isWord = /[a-zA-Z]/.test(glyph)
        const textClass = isWord ? 'text-xs font-semibold text-spade-cream' : 'text-base text-spade-cream'
        if (reaction.kind === 'aggregate') {
          return (
            <View key="aggregate" className="flex-row items-center gap-1 rounded-spade-pill border border-spade-cream/20 bg-spade-cream/10 px-2 py-0.5">
              <Text className={textClass}>{glyph}</Text>
              <Text className="text-xs font-semibold text-spade-cream">×{reaction.count}</Text>
            </View>
          )
        }
        return (
          <View key={reaction.seq} className="rounded-spade-pill border border-spade-cream/20 bg-spade-cream/10 px-2 py-0.5">
            <Text className={textClass}>{glyph}</Text>
          </View>
        )
      })}
    </View>
  )
}

function OpponentsColumn({ players, currentTurnName, emotes }: { players: Player[]; currentTurnName: string | null; emotes: Record<string, ActiveEmote> }) {
  const opponents = players.filter((p) => p.name !== 'You')
  if (opponents.length === 0) return null
  return (
    <>
      {opponents.map((player) => (
        <OpponentCompact key={player.name} player={player} isCurrentTurn={player.name === currentTurnName} emote={emotes[player.name]} />
      ))}
    </>
  )
}

function OpponentCompact({ player, isCurrentTurn, emote }: { player: Player; isCurrentTurn: boolean; emote: ActiveEmote | undefined }) {
  const ringClass = isCurrentTurn ? 'border-spade-gold' : 'border-spade-cream/10'
  const opacityClass = player.disconnected ? 'opacity-40' : ''
  return (
    <View className={`relative items-center gap-0.5 rounded-spade-md border ${ringClass} bg-spade-bg/50 px-1 py-1 ${opacityClass}`}>
      <EmoteBubble emote={emote} />
      <Avatar avatarUrl={player.avatarUrl} initials={player.initials} tone={player.tone} size={28} />
      <Text className="max-w-[52px] text-[9px] font-medium text-spade-cream" numberOfLines={1}>{player.name}</Text>
      <View className="flex-row items-center gap-1">
        <Text className="text-[8px] text-spade-gray-3">H{player.cardsLeft}</Text>
        <Text className="text-[8px] text-spade-gray-3">F{player.faceDownCount}</Text>
      </View>
    </View>
  )
}

function OpponentsRow({ players, currentTurnName, emotes }: { players: Player[]; currentTurnName: string | null; emotes: Record<string, ActiveEmote> }) {
  const opponents = players.filter((p) => p.name !== 'You')
  if (opponents.length === 0) return null
  return (
    <View className="w-full flex-row items-end justify-center gap-3">
      {opponents.map((player) => (
        <OpponentCard key={player.name} player={player} isCurrentTurn={player.name === currentTurnName} emote={emotes[player.name]} />
      ))}
    </View>
  )
}

function OpponentCard({ player, isCurrentTurn, emote }: { player: Player; isCurrentTurn: boolean; emote: ActiveEmote | undefined }) {
  const ringClass = isCurrentTurn ? 'border-spade-gold' : 'border-spade-cream/10'
  const opacityClass = player.disconnected ? 'opacity-50' : ''
  return (
    <View className={`relative items-center gap-1.5 rounded-spade-lg border ${ringClass} bg-spade-bg/50 px-3 py-2 ${opacityClass}`}>
      <EmoteBubble emote={emote} />
      <Avatar avatarUrl={player.avatarUrl} initials={player.initials} tone={player.tone} size={36} />
      <Text className="max-w-[80px] text-xs font-medium text-spade-cream" numberOfLines={1}>{player.name}</Text>
      <View className="flex-row items-center gap-2">
        <Text className="text-[10px] text-spade-gray-3">H {player.cardsLeft}</Text>
        <Text className="text-[10px] text-spade-gray-3">F {player.faceDownCount}</Text>
      </View>
      {player.disconnected ? <Text className="text-[9px] text-red-400">Offline</Text> : null}
    </View>
  )
}

function PlayerHandStrip({
  cards,
  interactive,
  onCardPress,
  isMyTurn,
  faceDownMode,
  onSelectFaceDown,
  onConfirmFaceDown,
  hasFaceDownSelection,
}: {
  cards: Card[]
  interactive: boolean
  onCardPress: (card: Card) => void
  isMyTurn: boolean
  faceDownMode: boolean
  onSelectFaceDown: (card: Card) => void
  onConfirmFaceDown: () => void
  hasFaceDownSelection: boolean
}) {
  if (cards.length === 0) return null
  const cardsInteractive = interactive || faceDownMode

  const handlePress = (card: Card) => {
    if (faceDownMode) {
      onSelectFaceDown(card)
      return
    }
    if (card.playable) {
      onCardPress(card)
    }
  }

  return (
    <View className="flex-row items-center gap-2 px-2 py-2">
      <ScrollView 
        horizontal 
        showsHorizontalScrollIndicator={false}
        contentContainerStyle={{ gap: 8, paddingHorizontal: 4 }}
        className="flex-1"
      >
        {cards.map((card, index) => {
          const clickable = faceDownMode || (interactive && card.playable)
          return (
            <CardFace
              key={`${card.rank}-${card.suit}-${index}`}
              card={card}
              size="landscape"
              interactive={cardsInteractive}
              onPress={clickable ? () => handlePress(card) : undefined}
              accessibilityLabel={faceDownMode ? `Select ${card.rank} of ${card.suit} for face down` : undefined}
            />
          )
        })}
      </ScrollView>
      {faceDownMode ? (
        <Button onPress={onConfirmFaceDown} disabled={!hasFaceDownSelection}>Place</Button>
      ) : null}
    </View>
  )
}

function PlayerHand({
  cards,
  interactive,
  onCardPress,
  isMyTurn,
  faceDownMode,
  onSelectFaceDown,
  onConfirmFaceDown,
  hasFaceDownSelection,
}: {
  cards: Card[]
  interactive: boolean
  onCardPress: (card: Card) => void
  isMyTurn: boolean
  faceDownMode: boolean
  onSelectFaceDown: (card: Card) => void
  onConfirmFaceDown: () => void
  hasFaceDownSelection: boolean
}) {
  if (cards.length === 0) return null
  const cardsInteractive = interactive || faceDownMode

  const handlePress = (card: Card) => {
    if (faceDownMode) {
      onSelectFaceDown(card)
      return
    }
    if (card.playable) {
      onCardPress(card)
    }
  }

  return (
    <View className="w-full">
      <View className="flex-row items-center justify-between px-1 pb-1">
        <Text className="text-xs font-medium text-spade-cream/70">Your hand</Text>
        <Text className="font-mono text-[10px] text-spade-gray-3">{cards.length} cards</Text>
      </View>
      <View className="flex-row flex-wrap items-end justify-center gap-1 pb-2 pt-2">
        {cards.map((card, index) => {
          const clickable = faceDownMode || (interactive && card.playable)
          return (
            <CardFace
              key={`${card.rank}-${card.suit}-${index}`}
              card={card}
              size="md"
              interactive={cardsInteractive}
              onPress={clickable ? () => handlePress(card) : undefined}
              accessibilityLabel={faceDownMode ? `Select ${card.rank} of ${card.suit} for face down` : undefined}
            />
          )
        })}
      </View>
      {faceDownMode ? (
        <View className="items-center gap-2">
          <Text className="text-center text-xs text-spade-gold/80">No valid moves - pick a card to place face down as a penalty.</Text>
          <Button onPress={onConfirmFaceDown} disabled={!hasFaceDownSelection}>Place face-down</Button>
        </View>
      ) : isMyTurn ? (
        <Text className="text-center text-xs text-spade-gold/80">Play a highlighted card to extend a suit</Text>
      ) : null}
    </View>
  )
}

function GameOverPanel({ roomId, game }: { roomId: string | undefined; game: GameSocketState }) {
  const router = useRouter()
  const hasSharedWin = game.results.filter((result) => result.winner).length > 1
  const winnerLabel = hasSharedWin ? 'Shared winner' : 'Winner'
  const rematchProgress = (game.rematchVotes / game.rematchTotal) * 100
  const scores = game.results.map((result) => ({
    rank: result.rank,
    player: result.player,
    cardsLeft: 0,
    penalty: result.penalty,
    result: result.winner ? winnerLabel : 'Finished',
    winner: result.winner,
  }))

  return (
    <View className="flex-1 bg-spade-bg">
      <SceneShell
        title="Results and rematch"
        eyebrow={game.practiceMode ? 'Practice Mode' : roomId ? `Room ${roomId}` : 'Game over'}
        action={<Badge tone="winner">{game.practiceMode ? 'Practice' : 'Round over'}</Badge>}
      >
        <View className="gap-4">
          <View className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
            <Text className="text-lg font-medium text-spade-cream">Final board</Text>
            <Text className="mt-1 text-sm text-spade-gray-2">The completed sequences, including any suits closed with an Ace.</Text>
            <View className="mt-4">
              <GameBoard rows={game.boardRows} />
            </View>
          </View>

          <ScoreTable scores={scores} winnerLabel={winnerLabel} />
          <MatchStatsCard results={game.results} myDisplayName={game.myDisplayName} practiceMode={game.practiceMode} />
          <RevealedPenaltyCards results={game.results} />

          <View className="rounded-spade-lg border border-spade-gold/30 bg-spade-gold/10 p-4">
            <Text className="text-lg font-medium text-spade-cream">Rematch vote</Text>
            <Text className="mt-1 text-sm text-spade-gray-2">
              {game.practiceMode
                ? 'Practice games are not saved to history or stats. Vote to play another round, or head back to the lobby.'
                : 'The game restarts in the same room once every player votes for a rematch.'}
            </Text>
            <View className="mt-4 gap-2">
              <Button onPress={game.sendRematchVote}>Vote rematch</Button>
              <Button variant="secondary" onPress={() => router.replace('/(app)/lobby')}>Leave room</Button>
              {game.practiceMode ? null : (
                <Button variant="ghost" onPress={() => router.push('/(app)/history')}>View history</Button>
              )}
            </View>
            <View className="mt-4 h-2 overflow-hidden rounded-full bg-spade-bg/70">
              <View className="h-full rounded-full bg-spade-gold-light" style={{ width: `${rematchProgress}%` }} />
            </View>
            <Text className="mt-2 font-mono text-xs text-spade-gold-light">{game.rematchVotes} / {game.rematchTotal} voted</Text>
            <View className="mt-4 gap-2">
              {game.players.map((player) => (
                <View key={player.name} className="flex-row items-center justify-between gap-3 rounded-spade-md border border-spade-cream/10 bg-spade-bg/55 px-3 py-2">
                  <Text className="flex-1 text-sm text-spade-cream" numberOfLines={1}>{player.name}</Text>
                  <Badge tone={player.votedRematch ? 'playing' : 'waiting'}>{player.votedRematch ? 'Voted' : 'Waiting'}</Badge>
                </View>
              ))}
            </View>
          </View>

          <ToastStack toasts={game.toasts} />
        </View>
      </SceneShell>
    </View>
  )
}

function MatchStatsCard({ results, myDisplayName, practiceMode }: { results: GameResult[]; myDisplayName: string | null; practiceMode: boolean }) {
  if (practiceMode || !myDisplayName) return null
  const myResult = results.find((r) => r.player === myDisplayName)
  if (!myResult || myResult.xpDelta === undefined) return null

  const ratingDelta = myResult.ratingDelta ?? 0
  const ratingSign = ratingDelta >= 0 ? '+' : ''

  return (
    <View className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
      <Text className="text-lg font-medium text-spade-cream">Your match rewards</Text>
      <View className="mt-3 flex-row gap-3">
        <View className="flex-1 rounded-spade-md border border-spade-cream/10 bg-spade-bg/45 p-3">
          <Text className="font-mono text-[10px] uppercase text-spade-gray-3">XP gained</Text>
          <Text className="mt-1 text-lg font-medium text-spade-gold-light">+{myResult.xpDelta}</Text>
          <Text className="mt-0.5 font-mono text-xs text-spade-gray-2">Level {myResult.level}</Text>
        </View>
        <View className="flex-1 rounded-spade-md border border-spade-cream/10 bg-spade-bg/45 p-3">
          <Text className="font-mono text-[10px] uppercase text-spade-gray-3">Rating</Text>
          <Text className={`mt-1 text-lg font-medium ${ratingDelta > 0 ? 'text-green-400' : ratingDelta < 0 ? 'text-red-400' : 'text-spade-cream'}`}>
            {ratingSign}{ratingDelta}
          </Text>
          <Text className="mt-0.5 font-mono text-xs text-spade-gray-2">{myResult.ratingAfter ?? '—'}</Text>
        </View>
      </View>
    </View>
  )
}

function RevealedPenaltyCards({ results }: { results: GameResult[] }) {
  return (
    <View className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
      <Text className="text-lg font-medium text-spade-cream">Revealed penalty cards</Text>
      <Text className="mt-1 text-sm text-spade-gray-2">Face-down values are shown after the round ends.</Text>
      <View className="mt-4 gap-3">
        {results.map((result) => (
          <View
            key={result.player}
            className={`rounded-spade-md border p-3 ${result.winner ? 'border-spade-gold/40 bg-spade-gold/10' : 'border-spade-cream/10 bg-spade-bg/45'}`}
          >
            <View className="mb-3 flex-row items-center justify-between gap-3">
              <View>
                <Text className="font-medium text-spade-cream">{result.player}</Text>
                <Text className="font-mono text-xs text-spade-gray-2">Rank {result.rank} - {result.penalty} penalty</Text>
              </View>
              {result.winner ? <Badge tone="winner">Winner</Badge> : null}
            </View>
            <View className="flex-row flex-wrap gap-2">
              {result.faceDownCards.length === 0 ? (
                <Text className="text-sm text-spade-gray-2">No penalty cards</Text>
              ) : null}
              {result.faceDownCards.map((card) => (
                <View key={`${result.player}-${card.rank}-${card.suit}`} className="flex-row items-center gap-2 rounded-spade-sm border border-spade-cream/10 bg-spade-bg/70 px-2 py-1">
                  <CardFace card={card} size="sm" interactive={false} />
                  <View className="gap-1">
                    <Text className="text-xs text-spade-cream">{card.rank} of {card.suit}</Text>
                    <Text className="font-mono text-xs text-spade-gold-light">+{card.points}</Text>
                  </View>
                </View>
              ))}
            </View>
          </View>
        ))}
      </View>
    </View>
  )
}

function useTurnClock(turnEndsAt: string | null): { label: string; percentRemaining: number } | null {
  const [, tick] = useState(0)

  useEffect(() => {
    if (!turnEndsAt) return undefined
    const interval = setInterval(() => {
      tick((value) => value + 1)
    }, 1000)
    return () => clearInterval(interval)
  }, [turnEndsAt])

  return turnEndsAt ? getTurnClock(turnEndsAt) : null
}

function getTurnClock(turnEndsAt: string): { label: string; percentRemaining: number } {
  const endsAt = Date.parse(turnEndsAt)
  if (Number.isNaN(endsAt)) {
    return { label: 'Live', percentRemaining: 100 }
  }
  const seconds = Math.max(0, Math.ceil((endsAt - Date.now()) / 1000))
  return {
    label: `00:${String(seconds).padStart(2, '0')}`,
    percentRemaining: Math.max(0, Math.min(100, Math.round((seconds / 60) * 100))),
  }
}
