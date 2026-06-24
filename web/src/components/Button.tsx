import type { ButtonHTMLAttributes, ReactNode } from 'react'

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger'
type ButtonSize = 'md' | 'sm' | 'xs'

const variantClasses: Record<ButtonVariant, string> = {
  primary: 'bg-spade-gold text-[#1a0e00] hover:bg-[#d9a030]',
  secondary: 'border border-spade-cream/18 bg-transparent text-spade-cream hover:bg-spade-cream/8',
  ghost: 'border border-spade-gold text-spade-gold-light hover:bg-spade-gold/10',
  danger: 'bg-spade-red text-white hover:bg-[#922b21]',
}

// Size controls vertical metrics so a button can sit cleanly inside dense rows
// (xs) without breaking row height. md matches the previous default exactly.
const sizeClasses: Record<ButtonSize, string> = {
  md: 'min-h-9 rounded-spade-md px-4 py-2 text-sm font-medium',
  sm: 'min-h-7 rounded-spade-md px-3 py-1 text-xs font-medium',
  xs: 'rounded-spade-sm px-2 py-0 text-xs font-medium leading-4',
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode
  variant?: ButtonVariant
  size?: ButtonSize
}

export function Button({
  children,
  variant = 'primary',
  size = 'md',
  className = '',
  ...props
}: ButtonProps) {
  return (
    <button
      className={`inline-flex items-center justify-center transition active:scale-95 disabled:cursor-not-allowed disabled:opacity-40 ${sizeClasses[size]} ${variantClasses[variant]} ${className}`}
      {...props}
    >
      {children}
    </button>
  )
}
