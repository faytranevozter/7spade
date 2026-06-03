import { useEffect, useMemo, useState } from 'react'
import { Text, View } from 'react-native'
import { useLocalSearchParams, useRouter } from 'expo-router'
import { Badge } from '../../../src/components/Badge'
import { Button } from '../../../src/components/Button'
import { Avatar } from '../../../src/components/Avatar'
import { EmoteBubble } from '../../../src/components/EmoteBubble'
import { EmotePicker } from '../../../src/components/EmotePicker'
import { SceneShell } from '../../../src/components/SceneShell'
import { ToastStack } from '../../../src/components/ToastStack'
import { ApiError } from '../../../src/api/client'
import { getRoom, type RoomDto } from '../../../src/api/lobby'
import { useAuth } from '../../../src/hooks/useAuth'
import { useGameSocket } from '../../../src/hooks/useGameSocket'
import { useSound } from '../../../src/hooks/useSound'
import { initialsForName } from '../../../src/game/cards'

const connectionTone = {
  idle: 'waiting',
  connecting: 'waiting',
  open: 'playing',
  closed: 'danger',
  error: 'danger',
} as const

function getErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return fallback
}

export default function WaitingRoomScreen() {
  const { id } = useLocalSearchParams<{ id: string }>()
  const roomId = id
  const router = useRouter()
  const { token } = useAuth()
  const game = useGameSocket(roomId, token)
  const { unlock: unlockSound } = useSound()
  const [roomDetails, setRoomDetails] = useState<RoomDto | null>(null)
  const [roomError, setRoomError] = useState<string | null>(null)

  useEffect(() => {
    if (!roomId || !token) return
    let cancelled = false
    getRoom(token, roomId)
      .then((data) => {
        if (!cancelled) setRoomDetails(data)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        if (err instanceof ApiError && err.statusCode === 404) {
          router.replace('/(app)/lobby')
          return
        }
        setRoomError(getErrorMessage(err, 'Failed to load room'))
      })
    return () => {
      cancelled = true
    }
  }, [roomId, token, router])

  // Once the game starts the WS hook flips phase to 'playing'. Hand off to the
  // live game screen so its own socket mounts cleanly.
  useEffect(() => {
    if (game.phase === 'playing' && roomId) {
      router.replace(`/(app)/game/${roomId}`)
    }
  }, [game.phase, roomId, router])

  const lobby = game.lobby
  const playerCount = lobby?.players.filter((p) => !p.disconnected).length ?? 0
  const minToStart = lobby?.minToStart ?? 2
  const maxPlayers = lobby?.maxPlayers ?? 4
  const slots = useMemo(() => {
    const filled = lobby?.players ?? []
    const placeholders: Array<null> = Array.from({ length: Math.max(0, maxPlayers - filled.length) }, () => null)
    return [...filled, ...placeholders]
  }, [lobby?.players, maxPlayers])

  const startBlockedReason = (() => {
    if (!lobby) return 'Connecting...'
    if (playerCount < minToStart) return `Need at least ${minToStart} players`
    if (!lobby.canStart) return 'Waiting for everyone to ready up'
    return null
  })()

  const inviteCode = roomDetails?.invite_code ?? ''

  const handleLeave = () => {
    game.sendLeave()
    router.replace('/(app)/lobby')
  }

  const action = (
    <View className="flex-row flex-wrap gap-2">
      <Badge tone={game.status === 'open' ? 'playing' : connectionTone[game.status]}>
        {game.status === 'open' ? 'Connected' : game.status}
      </Badge>
      <Badge tone="waiting">{`${playerCount} / ${maxPlayers}`}</Badge>
    </View>
  )

  return (
    <View className="flex-1 bg-spade-bg">
      <SceneShell title="Waiting room" eyebrow="Pre-game lobby" action={action}>
        <View className="gap-4">
          <View className="rounded-spade-lg border border-spade-cream/10 bg-spade-bg/55 p-4">
            <Text className="text-lg font-medium text-spade-cream">Invite</Text>
            <Text className="mt-1 text-sm text-spade-gray-2">
              Share this code so up to {maxPlayers} players can join. The host can start with at least {minToStart}; remaining seats fill with bots.
            </Text>
            <View className="mt-4 flex-row flex-wrap items-center gap-3">
              <Text className="rounded-spade-md border border-spade-gold/40 bg-spade-gold/10 px-4 py-2 font-mono text-lg tracking-widest text-spade-gold-light">
                {inviteCode || '......'}
              </Text>
              {roomDetails?.turn_timer_seconds ? (
                <Badge tone="waiting">{`Timer: ${roomDetails.turn_timer_seconds}s`}</Badge>
              ) : null}
            </View>
            {roomError ? <Text className="mt-3 text-xs text-spade-red">{roomError}</Text> : null}
          </View>

          <View className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
            <Text className="text-lg font-medium text-spade-cream">Players</Text>
            <Text className="mt-1 text-sm text-spade-gray-2">
              The host is auto-ready. Everyone else needs to mark ready before the game can start.
            </Text>
            <View className="mt-4 gap-2">
              {slots.map((player, index) => (
                <View
                  key={player ? player.displayName : `empty-${index}`}
                  className={`flex-row items-center justify-between gap-3 rounded-spade-md border border-spade-cream/10 bg-spade-bg/55 px-3 py-2 ${player?.disconnected ? 'opacity-55' : ''}`}
                >
                  <View className="flex-1 flex-row items-center gap-3">
                    <View className="relative">
                      {player ? <EmoteBubble emote={game.emotes[player.displayName]} /> : null}
                      {player ? (
                        <Avatar avatarUrl={player.avatarUrl} initials={initialsForName(player.displayName)} tone="green" size={36} />
                      ) : (
                        <View className="size-9 items-center justify-center rounded-full bg-spade-green-mid">
                          <Text className="text-sm text-spade-cream">-</Text>
                        </View>
                      )}
                    </View>
                    <View className="flex-1">
                      <Text className="text-sm font-medium text-spade-cream" numberOfLines={1}>
                        {player ? player.displayName : 'Waiting for player...'}
                      </Text>
                      <Text className="font-mono text-[11px] text-spade-gray-3">
                        {player ? `Slot ${index + 1}` : `Slot ${index + 1} - open`}
                      </Text>
                    </View>
                  </View>
                  <View className="flex-row items-center gap-2">
                    {player?.isHost ? <Badge tone="winner">Host</Badge> : null}
                    {player ? (
                      player.disconnected ? (
                        <Badge tone="danger">Offline</Badge>
                      ) : (
                        <Badge tone={player.ready ? 'playing' : 'waiting'}>{player.ready ? 'Ready' : 'Not ready'}</Badge>
                      )
                    ) : null}
                  </View>
                </View>
              ))}
            </View>
          </View>

          <View className="gap-3 rounded-spade-lg border border-spade-cream/10 bg-spade-bg/55 p-4">
            <Text className="text-lg font-medium text-spade-cream">Your status</Text>
            {game.isHost ? (
              <>
                <Text className="text-sm text-spade-gray-2">
                  You are the host. Empty seats fill with bots when the game starts.
                </Text>
                <Button onPress={() => { unlockSound(); game.sendStartGame() }} disabled={!lobby?.canStart}>
                  Start game
                </Button>
                {startBlockedReason ? <Text className="font-mono text-[11px] text-spade-gray-3">{startBlockedReason}</Text> : null}
              </>
            ) : (
              <>
                <Text className="text-sm text-spade-gray-2">
                  Mark yourself ready when set. The host starts the round once everyone is ready.
                </Text>
                <Button onPress={() => { unlockSound(); game.sendSetReady(!game.iAmReady) }}>
                  {game.iAmReady ? 'Cancel ready' : 'Mark ready'}
                </Button>
              </>
            )}
            <Button variant="danger" onPress={handleLeave}>Leave room</Button>
            <View className="flex-row items-center justify-between gap-3 border-t border-spade-cream/10 pt-3">
              <Text className="text-sm text-spade-gray-2">Send an emote</Text>
              <EmotePicker onSelect={game.sendEmote} />
            </View>
          </View>

          <ToastStack toasts={game.toasts} />
        </View>
      </SceneShell>
    </View>
  )
}
