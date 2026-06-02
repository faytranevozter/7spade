import { Text, View } from 'react-native'
import type { AchievementDto } from '../api/achievements'
import { achievements as fallbackCatalog } from '../game/achievements'
import { SectionPanel } from './SectionPanel'

type BadgeGridProps = {
  catalog: AchievementDto[]
  earned: string[]
  earnedAt?: Record<string, string>
}

// Native port of web/src/components/BadgeGrid.tsx. Renders the full achievement
// catalog, highlighting earned badges and dimming locked ones.
export function BadgeGrid({ catalog, earned }: BadgeGridProps) {
  const visibleCatalog = catalog.length > 0 ? catalog : fallbackCatalog
  const earnedSet = new Set(earned)
  const earnedCount = visibleCatalog.filter((a) => earnedSet.has(a.id)).length

  return (
    <SectionPanel title="Achievements" eyebrow={`${earnedCount} / ${visibleCatalog.length} unlocked`}>
      <View className="flex-row flex-wrap gap-3">
        {visibleCatalog.map((a) => {
          const unlocked = earnedSet.has(a.id)
          return (
            <View
              key={a.id}
              className={`min-w-[45%] flex-1 items-center gap-1 rounded-spade-lg border px-3 py-3 ${
                unlocked ? 'border-spade-gold/40 bg-spade-gold/10' : 'border-spade-cream/10 bg-spade-bg/40 opacity-50'
              }`}
            >
              <Text className="text-2xl">{a.icon}</Text>
              <Text className="text-center text-xs font-medium text-spade-cream">{a.name}</Text>
              <Text className="text-center font-mono text-[10px] text-spade-gray-3">{a.description}</Text>
            </View>
          )
        })}
      </View>
    </SectionPanel>
  )
}
