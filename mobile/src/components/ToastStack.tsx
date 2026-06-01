import { Text, View } from 'react-native'
import type { Toast, ToastTone } from '../types'

// Native port of web/src/components/ToastStack.tsx.
const containerClasses: Record<ToastTone, string> = {
  success: 'border-spade-green-light/30 bg-spade-green-light/10',
  warn: 'border-spade-gold/30 bg-spade-gold/10',
  info: 'border-[#1e4080]/35 bg-[#1e4080]/15',
  error: 'border-spade-red/30 bg-spade-red/10',
}

const icons: Record<ToastTone, string> = {
  success: '✓',
  warn: '◷',
  info: '→',
  error: '×',
}

export function ToastStack({ toasts }: { toasts: Toast[] }) {
  if (toasts.length === 0) return null
  return (
    <View className="gap-2">
      {toasts.map((toast) => (
        <View key={toast.id} className={`flex-row items-start gap-3 rounded-spade-md border p-3 ${containerClasses[toast.tone]}`}>
          <Text className="text-sm text-spade-gold-light">{icons[toast.tone]}</Text>
          <View className="flex-1">
            <Text className="text-sm font-medium text-spade-cream">{toast.title}</Text>
            <Text className="mt-0.5 text-xs text-spade-gray-2">{toast.body}</Text>
          </View>
        </View>
      ))}
    </View>
  )
}
