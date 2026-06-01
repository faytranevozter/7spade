import { Pressable, Text, type PressableProps } from 'react-native'
import type { ReactNode } from 'react'

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger'

// Native port of web/src/components/Button.tsx. Same variants and disabled
// behaviour, rebuilt with Pressable + NativeWind classes.
const containerClasses: Record<ButtonVariant, string> = {
  primary: 'bg-spade-gold',
  secondary: 'border border-spade-cream/20 bg-transparent',
  ghost: 'border border-spade-gold bg-transparent',
  danger: 'bg-spade-red',
}

const labelClasses: Record<ButtonVariant, string> = {
  primary: 'text-[#1a0e00]',
  secondary: 'text-spade-cream',
  ghost: 'text-spade-gold-light',
  danger: 'text-white',
}

type ButtonProps = Omit<PressableProps, 'children'> & {
  children: ReactNode
  variant?: ButtonVariant
  className?: string
  textClassName?: string
}

export function Button({
  children,
  variant = 'primary',
  className = '',
  textClassName = '',
  disabled,
  ...props
}: ButtonProps) {
  return (
    <Pressable
      accessibilityRole="button"
      disabled={disabled}
      className={`min-h-11 flex-row items-center justify-center rounded-spade-md px-4 py-3 ${containerClasses[variant]} ${disabled ? 'opacity-40' : 'active:opacity-80'} ${className}`}
      {...props}
    >
      {typeof children === 'string' ? (
        <Text className={`text-sm font-medium ${labelClasses[variant]} ${textClassName}`}>{children}</Text>
      ) : (
        children
      )}
    </Pressable>
  )
}
