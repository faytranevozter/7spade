import type { ButtonHTMLAttributes, ReactNode } from 'react'

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger'

const variantClasses: Record<ButtonVariant, string> = {
  primary: 'bg-spade-gold text-[#1a0e00] hover:bg-[#d9a030]',
  secondary: 'border border-spade-cream/18 bg-transparent text-spade-cream hover:bg-spade-cream/8',
  ghost: 'border border-spade-gold text-spade-gold-light hover:bg-spade-gold/10',
  danger: 'bg-spade-red text-white hover:bg-[#922b21]',
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode
  variant?: ButtonVariant
}

export function Button({ children, variant = 'primary', className = '', ...props }: ButtonProps) {
  return (
    <button
      className={`inline-flex min-h-9 items-center justify-center rounded-spade-md px-4 py-2 text-sm font-medium transition active:scale-95 disabled:cursor-not-allowed disabled:opacity-40 ${variantClasses[variant]} ${className}`}
      {...props}
    >
      {children}
    </button>
  )
}
