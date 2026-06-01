import type { ReactNode } from 'react'
import { Text, View } from 'react-native'

type SectionPanelProps = {
  title: string
  eyebrow: string
  children: ReactNode
  action?: ReactNode
}

// Native port of web/src/components/SectionPanel.tsx.
export function SectionPanel({ title, eyebrow, children, action }: SectionPanelProps) {
  return (
    <View className="rounded-spade-xl border border-spade-green-light/25 bg-[#102316] p-4">
      <View className="mb-4 flex-row flex-wrap items-start justify-between gap-3 border-b border-spade-cream/10 pb-3">
        <View className="flex-1">
          <Text className="font-mono text-[11px] uppercase tracking-wider text-spade-gold">{eyebrow}</Text>
          <Text className="mt-1 text-2xl font-medium text-spade-cream">{title}</Text>
        </View>
        {action}
      </View>
      {children}
    </View>
  )
}
