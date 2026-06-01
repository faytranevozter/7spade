import type { ReactNode } from 'react'
import { Modal as RNModal, Pressable, Text, View } from 'react-native'

type ModalTone = 'default' | 'danger'

type ModalProps = {
  title: string
  eyebrow?: string
  description?: string
  tone?: ModalTone
  children?: ReactNode
  footer?: ReactNode
  visible?: boolean
  onClose?: () => void
}

// Native port of web/src/components/Modal.tsx, built on React Native's Modal.
// Tapping the backdrop or the close button dismisses it (mirrors the web
// backdrop-click + Escape affordances; there's no hardware Escape on mobile, but
// Android back is handled by onRequestClose).
const toneClasses: Record<ModalTone, string> = {
  default: 'border-spade-green-light/25',
  danger: 'border-spade-red/35',
}

export function Modal({
  title,
  eyebrow,
  description,
  tone = 'default',
  children,
  footer,
  visible = true,
  onClose,
}: ModalProps) {
  return (
    <RNModal visible={visible} transparent animationType="fade" onRequestClose={onClose}>
      <Pressable className="flex-1 items-center justify-center bg-black/60 p-4" onPress={onClose}>
        <Pressable
          className={`w-full max-w-[420px] rounded-spade-lg border ${toneClasses[tone]} bg-spade-gray-4 p-5`}
          onPress={(e) => e.stopPropagation()}
        >
          <View className="flex-row items-start justify-between gap-4">
            <View className="flex-1">
              {eyebrow ? (
                <Text className="mb-2 text-xs font-semibold uppercase tracking-wider text-spade-gold">{eyebrow}</Text>
              ) : null}
              <Text className="text-xl font-medium text-spade-cream">{title}</Text>
              {description ? (
                <Text className="mt-2 text-sm leading-5 text-spade-gray-2">{description}</Text>
              ) : null}
            </View>
            {onClose ? (
              <Pressable
                accessibilityRole="button"
                accessibilityLabel={`Close ${title}`}
                onPress={onClose}
                className="size-8 items-center justify-center rounded-spade-sm border border-spade-cream/15"
              >
                <Text className="text-spade-gray-2">×</Text>
              </Pressable>
            ) : null}
          </View>

          {children ? <View className="mt-5">{children}</View> : null}

          {footer ? <View className="mt-5 gap-2">{footer}</View> : null}
        </Pressable>
      </Pressable>
    </RNModal>
  )
}
