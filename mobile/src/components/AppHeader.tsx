import { Pressable, Text, View } from 'react-native'
import { useRouter } from 'expo-router'
import { SafeAreaView } from 'react-native-safe-area-context'
import { useAuth } from '../hooks/useAuth'
import { useSound } from '../hooks/useSound'
import { deleteLogout } from '../api/auth'

// AppHeader is the persistent top bar for authenticated screens, porting the
// web AppShell header: brand, primary nav (Lobby / My Games / Leaderboard),
// sound toggle, and sign out. Rendered by each screen so it sits inside the
// safe area.
export function AppHeader() {
  const router = useRouter()
  const { logout, refreshToken } = useAuth()
  const { muted, toggleMuted } = useSound()

  const handleSignOut = () => {
    const rt = refreshToken
    logout()
    // Best-effort server-side revoke; the local logout above is what matters.
    void deleteLogout(rt).catch(() => {})
  }

  return (
    <SafeAreaView edges={['top']} className="border-b border-spade-green-light/25 bg-spade-bg">
      <View className="flex-row items-center justify-between px-4 py-3">
        <Pressable className="flex-row items-center gap-2" onPress={() => router.replace('/(app)/lobby')}>
          <View className="size-9 items-center justify-center rounded-spade-lg bg-spade-gold">
            <Text className="text-lg text-[#1a0e00]">♠</Text>
          </View>
          <Text className="text-lg font-medium text-spade-cream">Seven Spade</Text>
        </Pressable>
        <View className="flex-row items-center gap-1">
          <NavButton label="Lobby" onPress={() => router.replace('/(app)/lobby')} />
          <NavButton label="Games" onPress={() => router.push('/(app)/history')} />
          <NavButton label="Ranks" onPress={() => router.push('/(app)/leaderboard')} />
          <Pressable
            accessibilityRole="button"
            accessibilityLabel={muted ? 'Unmute' : 'Mute'}
            onPress={toggleMuted}
            className="px-2 py-2"
          >
            <Text className="text-base">{muted ? '🔇' : '🔊'}</Text>
          </Pressable>
          <Pressable accessibilityRole="button" onPress={handleSignOut} className="px-2 py-2">
            <Text className="text-xs text-spade-gray-2">Exit</Text>
          </Pressable>
        </View>
      </View>
    </SafeAreaView>
  )
}

function NavButton({ label, onPress }: { label: string; onPress: () => void }) {
  return (
    <Pressable accessibilityRole="button" onPress={onPress} className="rounded-spade-pill px-2.5 py-2">
      <Text className="text-sm text-spade-gray-2">{label}</Text>
    </Pressable>
  )
}
