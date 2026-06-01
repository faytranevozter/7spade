import type { ReactNode } from 'react'
import { ScrollView, Text, View } from 'react-native'

type SceneShellProps = {
  title: string
  eyebrow: string
  children: ReactNode
  action?: ReactNode
}

// Native port of web/src/components/SceneShell.tsx. Wraps a screen in a
// scrollable, titled panel. The web header bar is provided by the navigator
// (app/(app)/_layout.tsx), so SceneShell just renders the page heading + body.
export function SceneShell({ title, eyebrow, children, action }: SceneShellProps) {
  return (
    <ScrollView className="flex-1 bg-spade-bg" contentContainerClassName="gap-4 px-4 py-5">
      <View className="flex-row flex-wrap items-end justify-between gap-3">
        <View className="flex-1">
          <Text className="font-mono text-xs uppercase tracking-wider text-spade-gold">{eyebrow}</Text>
          <Text className="mt-1 text-3xl font-medium text-spade-cream">{title}</Text>
        </View>
        {action ? <View className="flex-row flex-wrap gap-2">{action}</View> : null}
      </View>
      <View className="rounded-spade-xl border border-spade-green-light/25 bg-[#102316] p-4">
        {children}
      </View>
    </ScrollView>
  )
}
