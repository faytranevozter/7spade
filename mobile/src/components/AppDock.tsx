import { Pressable, Text, View } from 'react-native'
import { usePathname, useRouter } from 'expo-router'
import { SafeAreaView } from 'react-native-safe-area-context'

type DockTab = {
  label: string
  glyph: string
  href: '/(app)/lobby' | '/(app)/history' | '/(app)/leaderboard' | '/(app)/me'
  activePrefixes: string[]
}

const tabs: DockTab[] = [
  { label: 'Lobby', glyph: '♣', href: '/(app)/lobby', activePrefixes: ['/lobby', '/room', '/game', '/spectate'] },
  { label: 'Games', glyph: '♦', href: '/(app)/history', activePrefixes: ['/history'] },
  { label: 'Ranks', glyph: '♠', href: '/(app)/leaderboard', activePrefixes: ['/leaderboard'] },
  { label: 'Me', glyph: '♥', href: '/(app)/me', activePrefixes: ['/me', '/friends', '/profile'] },
]

export function AppDock() {
  const router = useRouter()
  const pathname = usePathname()

  return (
    <SafeAreaView pointerEvents="box-none" edges={['bottom', 'left', 'right']} className="absolute inset-x-0 bottom-0 z-20">
      <View pointerEvents="box-none" className="items-center px-4 pb-2">
        <View className="flex-row items-center gap-1.5 rounded-spade-xl border border-spade-gold/25 bg-[#07130d]/95 p-1.5 shadow-spade-card">
          {tabs.map((tab) => (
            <DockButton
              key={tab.href}
              tab={tab}
              active={tab.activePrefixes.some((prefix) => pathname.startsWith(prefix))}
              onPress={() => router.replace(tab.href)}
            />
          ))}
        </View>
      </View>
    </SafeAreaView>
  )
}

function DockButton({ tab, active, onPress }: { tab: DockTab; active: boolean; onPress: () => void }) {
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ selected: active }}
      onPress={onPress}
      className={`min-w-16 items-center rounded-spade-lg border px-3 py-1.5 ${
        active ? 'border-spade-gold-light bg-spade-gold' : 'border-spade-cream/10 bg-spade-green/40'
      }`}
    >
      <Text className={`text-base leading-5 ${active ? 'text-[#1a0e00]' : 'text-spade-gold-light'}`}>{tab.glyph}</Text>
      <Text className={`text-[10px] font-medium leading-4 ${active ? 'text-[#1a0e00]' : 'text-spade-gray-2'}`}>{tab.label}</Text>
    </Pressable>
  )
}
