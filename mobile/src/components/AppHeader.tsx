import { Pressable, Text, View } from 'react-native'
import { useRouter } from 'expo-router'
import { SafeAreaView } from 'react-native-safe-area-context'

// AppHeader is the persistent top bar for authenticated screens: brand + the
// primary nav (Lobby / Games / Ranks / Profile). The sound toggle and sign-out
// live on the Profile screen (`/(app)/me`) — putting them here overflowed the
// phone width and clipped them off the right edge.
export function AppHeader() {
  const router = useRouter()

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
          <NavButton label="Profile" onPress={() => router.push('/(app)/me')} />
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
