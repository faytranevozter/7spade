import { useCallback, useEffect, useState } from 'react'
import { Text, TextInput, View } from 'react-native'
import { useRouter } from 'expo-router'
import { Badge } from '../../src/components/Badge'
import { Button } from '../../src/components/Button'
import { Modal } from '../../src/components/Modal'
import { RoomCard } from '../../src/components/RoomCard'
import { SceneShell } from '../../src/components/SceneShell'
import { FriendsPanel } from '../../src/components/FriendsPanel'
import { ApiError } from '../../src/api/client'
import {
  getRooms,
  postJoinRoom,
  postRoom,
  type BotDifficulty,
  type RoomDto,
  type RoomVisibility,
} from '../../src/api/lobby'
import { getLiveGames, type LiveGameDto } from '../../src/api/liveGames'
import { useAuth } from '../../src/hooks/useAuth'
import { decodeJwtClaims } from '../../src/auth/claims'
import type { Room } from '../../src/types'

const TIMER_OPTIONS: ReadonlyArray<30 | 60 | 90 | 120> = [30, 60, 90, 120]
const BOT_DIFFICULTY_OPTIONS: ReadonlyArray<BotDifficulty> = ['easy', 'medium', 'hard']

function botDifficultyLabel(value: BotDifficulty): string {
  return value.charAt(0).toUpperCase() + value.slice(1)
}

function roomDtoToRoom(dto: RoomDto): Room {
  const fillStatus = dto.player_count >= 4 ? 'Full' : `${dto.player_count} / 4 players`
  return {
    name: dto.visibility === 'private' ? 'Private room' : 'Public room',
    code: dto.invite_code,
    players: `${dto.player_count} / 4`,
    status: dto.status === 'waiting' ? fillStatus : `Status: ${dto.status}`,
    timer: `${dto.turn_timer_seconds}s`,
    botDifficulty: botDifficultyLabel(dto.bot_difficulty),
    open: dto.status === 'waiting' && dto.player_count < 4,
    filledSeats: Math.min(dto.player_count, 4),
    maxSeats: 4,
    visibility: dto.visibility,
  }
}

function getErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return fallback
}

export default function LobbyScreen() {
  const router = useRouter()
  const { token } = useAuth()
  const isGuest = decodeJwtClaims(token).isGuest

  const [rooms, setRooms] = useState<RoomDto[]>([])
  const [isLoadingRooms, setIsLoadingRooms] = useState(false)
  const [listError, setListError] = useState<string | null>(null)
  const [liveGames, setLiveGames] = useState<LiveGameDto[]>([])

  const [visibility, setVisibility] = useState<RoomVisibility>('public')
  const [timer, setTimer] = useState<30 | 60 | 90 | 120>(60)
  const [botDifficulty, setBotDifficulty] = useState<BotDifficulty>('medium')
  const [isCreating, setIsCreating] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)
  const [showCreate, setShowCreate] = useState(false)

  const [inviteCode, setInviteCode] = useState('')
  const [isJoining, setIsJoining] = useState(false)
  const [joinError, setJoinError] = useState<string | null>(null)
  const [showJoin, setShowJoin] = useState(false)

  const [showPractice, setShowPractice] = useState(false)
  const [practiceTimer, setPracticeTimer] = useState<30 | 60 | 90 | 120>(60)
  const [practiceBotDifficulty, setPracticeBotDifficulty] = useState<BotDifficulty>('medium')
  const [isStartingPractice, setIsStartingPractice] = useState(false)
  const [practiceError, setPracticeError] = useState<string | null>(null)

  const [refreshNonce, setRefreshNonce] = useState(0)

  const loadRooms = useCallback(
    (background: boolean) => {
      let cancelled = false
      Promise.resolve()
        .then(() => {
          if (cancelled) return null
          if (!background) {
            setIsLoadingRooms(true)
            setListError(null)
          }
          return getRooms(token)
        })
        .then((data) => {
          if (cancelled || data === null) return
          setRooms(data)
          if (background) setListError(null)
        })
        .catch((err: unknown) => {
          if (cancelled) return
          setListError(getErrorMessage(err, 'Failed to load rooms'))
        })
        .finally(() => {
          if (cancelled || background) return
          setIsLoadingRooms(false)
        })
      return () => {
        cancelled = true
      }
    },
    [token],
  )

  useEffect(() => loadRooms(false), [loadRooms, refreshNonce])

  const loadLiveGames = useCallback(() => {
    let cancelled = false
    getLiveGames(token)
      .then((data) => {
        if (!cancelled) setLiveGames(data.games)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [token])

  useEffect(() => loadLiveGames(), [loadLiveGames, refreshNonce])

  useEffect(() => {
    const interval = setInterval(() => {
      loadRooms(true)
      loadLiveGames()
    }, 5000)
    return () => clearInterval(interval)
  }, [loadRooms, loadLiveGames])

  const handleCreateRoom = async () => {
    setCreateError(null)
    setIsCreating(true)
    try {
      const created = await postRoom(token, { visibility, turn_timer_seconds: timer, bot_difficulty: botDifficulty })
      setShowCreate(false)
      router.push(`/(app)/room/${created.id}`)
    } catch (err) {
      setCreateError(getErrorMessage(err, 'Failed to create room'))
    } finally {
      setIsCreating(false)
    }
  }

  const handleJoinByCode = async () => {
    const code = inviteCode.trim().toUpperCase()
    if (!code) {
      setJoinError('Enter an invite code')
      return
    }
    setJoinError(null)
    setIsJoining(true)
    try {
      const joined = await postJoinRoom(token, code)
      setShowJoin(false)
      router.push(`/(app)/room/${joined.id}`)
    } catch (err) {
      setJoinError(getErrorMessage(err, 'Failed to join room'))
    } finally {
      setIsJoining(false)
    }
  }

  const handleJoinPublic = async (room: RoomDto) => {
    setJoinError(null)
    try {
      const joined = await postJoinRoom(token, room.invite_code)
      router.push(`/(app)/room/${joined.id}`)
    } catch (err) {
      setJoinError(getErrorMessage(err, 'Failed to join room'))
    }
  }

  const handleStartPractice = async () => {
    setPracticeError(null)
    setIsStartingPractice(true)
    try {
      const created = await postRoom(token, {
        visibility: 'private',
        turn_timer_seconds: practiceTimer,
        bot_difficulty: practiceBotDifficulty,
        practice_mode: true,
      })
      setShowPractice(false)
      router.push(`/(app)/room/${created.id}`)
    } catch (err) {
      setPracticeError(getErrorMessage(err, 'Failed to start practice'))
    } finally {
      setIsStartingPractice(false)
    }
  }

  const openRoomCount = rooms.filter((room) => room.status === 'waiting' && room.player_count < 4).length

  return (
    <View className="flex-1 bg-spade-bg">
      <SceneShell
        title="Game lobby"
        eyebrow="Lobby"
        action={
          <View className="flex-row flex-wrap items-center gap-2">
            <Badge tone="waiting">{`${openRoomCount} waiting`}</Badge>
            <Button onPress={() => { setPracticeError(null); setShowPractice(true) }}>Practice</Button>
            <Button variant="secondary" onPress={() => { setCreateError(null); setShowCreate(true) }}>Create</Button>
            <Button variant="secondary" onPress={() => { setJoinError(null); setShowJoin(true) }}>Join code</Button>
          </View>
        }
      >
        <View className="gap-3">
          <Text className="text-sm font-medium text-spade-gray-2">Public rooms</Text>
          {listError ? <Text className="text-xs text-spade-red">{listError}</Text> : null}
          {!isLoadingRooms && rooms.length === 0 && !listError ? (
            <View className="rounded-spade-lg border border-dashed border-spade-cream/15 bg-spade-bg/40 p-8">
              <Text className="text-center text-sm text-spade-gray-2">No public rooms waiting.</Text>
              <Text className="mt-1 text-center text-xs text-spade-gray-3">Create one to get the table started.</Text>
            </View>
          ) : null}
          {rooms.map((room) => (
            <RoomCard key={room.id} room={roomDtoToRoom(room)} onJoin={() => void handleJoinPublic(room)} />
          ))}

          {liveGames.length > 0 ? (
            <View className="mt-2 gap-2">
              <Text className="text-sm font-medium text-spade-gray-2">Watch live</Text>
              {liveGames.map((live) => (
                <View
                  key={live.room_id}
                  className="flex-row items-center justify-between gap-3 rounded-spade-lg border border-spade-cream/10 bg-spade-bg/55 px-3 py-2"
                >
                  <View className="flex-1">
                    <Text className="text-sm font-medium text-spade-cream" numberOfLines={1}>
                      {live.players.map((p) => p.display_name).join(', ') || 'In progress'}
                    </Text>
                    <Text className="font-mono text-[11px] text-spade-gray-3">
                      {live.player_count} {live.player_count === 1 ? 'player' : 'players'} - in progress
                    </Text>
                  </View>
                  <Button variant="secondary" onPress={() => router.push(`/(app)/spectate/${live.room_id}`)}>Watch</Button>
                </View>
              ))}
            </View>
          ) : null}

          {!isGuest ? (
            <View className="mt-2">
              <FriendsPanel token={token} refreshNonce={refreshNonce} />
            </View>
          ) : null}
        </View>
      </SceneShell>

      {showCreate ? (
        <Modal
          title="Create room"
          eyebrow="New table"
          description="Pick how players join and how long each turn lasts."
          onClose={() => setShowCreate(false)}
        >
          <View className="gap-5">
            <View className="gap-2">
              <Text className="text-xs font-medium uppercase text-spade-gray-2">Visibility</Text>
              <View className="flex-row gap-2">
                {(['public', 'private'] as const).map((value) => (
                  <Button
                    key={value}
                    variant={visibility === value ? 'primary' : 'secondary'}
                    className="flex-1"
                    onPress={() => setVisibility(value)}
                  >
                    {value === 'public' ? 'Public' : 'Private'}
                  </Button>
                ))}
              </View>
            </View>
            <View className="gap-2">
              <Text className="text-xs font-medium uppercase text-spade-gray-2">Turn timer</Text>
              <View className="flex-row gap-2">
                {TIMER_OPTIONS.map((value) => (
                  <Button
                    key={value}
                    variant={timer === value ? 'primary' : 'secondary'}
                    className="flex-1"
                    onPress={() => setTimer(value)}
                  >
                    {`${value}s`}
                  </Button>
                ))}
              </View>
            </View>
            <View className="gap-2">
              <Text className="text-xs font-medium uppercase text-spade-gray-2">Bot difficulty</Text>
              <View className="flex-row gap-2">
                {BOT_DIFFICULTY_OPTIONS.map((value) => (
                  <Button
                    key={value}
                    variant={botDifficulty === value ? 'primary' : 'secondary'}
                    className="flex-1"
                    onPress={() => setBotDifficulty(value)}
                  >
                    {botDifficultyLabel(value)}
                  </Button>
                ))}
              </View>
            </View>
            {createError ? <Text className="text-xs text-spade-red">{createError}</Text> : null}
            <View className="gap-2">
              <Button onPress={handleCreateRoom} disabled={isCreating}>
                {isCreating ? 'Creating...' : 'Create'}
              </Button>
              <Button variant="secondary" onPress={() => setShowCreate(false)}>Cancel</Button>
            </View>
          </View>
        </Modal>
      ) : null}

      {showPractice ? (
        <Modal
          title="Practice mode"
          eyebrow="Solo vs bots"
          description="Play a private game against three bots. Practice games are not saved to history or stats."
          onClose={() => setShowPractice(false)}
        >
          <View className="gap-5">
            <View className="gap-2">
              <Text className="text-xs font-medium uppercase text-spade-gray-2">Bot difficulty</Text>
              <View className="flex-row gap-2">
                {BOT_DIFFICULTY_OPTIONS.map((value) => (
                  <Button
                    key={value}
                    variant={practiceBotDifficulty === value ? 'primary' : 'secondary'}
                    className="flex-1"
                    onPress={() => setPracticeBotDifficulty(value)}
                  >
                    {botDifficultyLabel(value)}
                  </Button>
                ))}
              </View>
            </View>
            <View className="gap-2">
              <Text className="text-xs font-medium uppercase text-spade-gray-2">Turn timer</Text>
              <View className="flex-row gap-2">
                {TIMER_OPTIONS.map((value) => (
                  <Button
                    key={value}
                    variant={practiceTimer === value ? 'primary' : 'secondary'}
                    className="flex-1"
                    onPress={() => setPracticeTimer(value)}
                  >
                    {`${value}s`}
                  </Button>
                ))}
              </View>
            </View>
            {practiceError ? <Text className="text-xs text-spade-red">{practiceError}</Text> : null}
            <View className="gap-2">
              <Button onPress={handleStartPractice} disabled={isStartingPractice}>
                {isStartingPractice ? 'Starting...' : 'Start practice'}
              </Button>
              <Button variant="secondary" onPress={() => setShowPractice(false)}>Cancel</Button>
            </View>
          </View>
        </Modal>
      ) : null}

      {showJoin ? (
        <Modal
          title="Join by code"
          eyebrow="Private room"
          description="Enter the invite code shared by the host."
          onClose={() => setShowJoin(false)}
        >
          <View className="gap-4">
            <View className="gap-1.5">
              <Text className="text-xs font-medium uppercase text-spade-gray-2">Invite code</Text>
              <TextInput
                value={inviteCode}
                onChangeText={(v) => setInviteCode(v.toUpperCase())}
                placeholder="XKQP7A"
                placeholderTextColor="#9c958966"
                autoCapitalize="characters"
                autoCorrect={false}
                maxLength={8}
                className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 font-mono text-sm tracking-wider text-spade-cream"
              />
            </View>
            {joinError ? <Text className="text-xs text-spade-red">{joinError}</Text> : null}
            <View className="gap-2">
              <Button onPress={handleJoinByCode} disabled={isJoining}>
                {isJoining ? 'Joining...' : 'Join with code'}
              </Button>
              <Button variant="secondary" onPress={() => setShowJoin(false)}>Cancel</Button>
            </View>
          </View>
        </Modal>
      ) : null}
    </View>
  )
}
