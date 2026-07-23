import type { ReactNode } from 'react'

const providerMeta: Record<
  string,
  { label: string; className: string; icon: ReactNode }
> = {
  google: {
    label: 'Google',
    className: 'border-white/15 bg-white text-[#1f1f1f]',
    icon: (
      <svg viewBox="0 0 24 24" className="size-3.5" aria-hidden="true">
        <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.56c2.08-1.92 3.28-4.74 3.28-8.1z" />
        <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.56-2.77c-.99.66-2.25 1.06-3.72 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84A11 11 0 0 0 12 23z" />
        <path fill="#FBBC05" d="M5.84 14.1A6.6 6.6 0 0 1 5.5 12c0-.73.13-1.44.34-2.1V7.07H2.18A11 11 0 0 0 1 12c0 1.78.43 3.46 1.18 4.94l3.66-2.84z" />
        <path fill="#EA4335" d="M12 5.38c1.62 0 3.07.56 4.21 1.65l3.15-3.15C17.45 2.09 14.97 1 12 1A11 11 0 0 0 2.18 7.07l3.66 2.83C6.71 7.31 9.14 5.38 12 5.38z" />
      </svg>
    ),
  },
  github: {
    label: 'GitHub',
    className: 'border-[#30363d] bg-[#24292f] text-white',
    icon: (
      <svg viewBox="0 0 24 24" className="size-3.5 fill-current" aria-hidden="true">
        <path d="M12 .5A11.5 11.5 0 0 0 .5 12c0 5.08 3.29 9.39 7.86 10.91.58.1.79-.25.79-.56v-1.97c-3.2.7-3.87-1.54-3.87-1.54-.52-1.33-1.28-1.69-1.28-1.69-1.05-.72.08-.71.08-.71 1.16.08 1.77 1.19 1.77 1.19 1.03 1.77 2.7 1.26 3.36.96.1-.75.4-1.26.73-1.55-2.55-.29-5.24-1.27-5.24-5.66 0-1.25.45-2.27 1.18-3.07-.12-.29-.51-1.46.11-3.04 0 0 .96-.31 3.16 1.17a10.94 10.94 0 0 1 5.75 0c2.2-1.48 3.16-1.17 3.16-1.17.62 1.58.23 2.75.11 3.04.74.8 1.18 1.82 1.18 3.07 0 4.4-2.69 5.36-5.25 5.65.41.36.78 1.06.78 2.13v3.16c0 .31.21.66.8.55A11.5 11.5 0 0 0 23.5 12 11.5 11.5 0 0 0 12 .5z" />
      </svg>
    ),
  },
  telegram: {
    label: 'Telegram',
    className: 'border-[#1a8bc7] bg-[#2AABEE] text-white',
    icon: (
      <svg viewBox="0 0 24 24" className="size-3.5 fill-current" aria-hidden="true">
        <path d="M12 0C5.373 0 0 5.373 0 12s5.373 12 12 12 12-5.373 12-12S18.627 0 12 0zm5.894 8.221-1.97 9.28c-.145.658-.537.818-1.084.508l-3-2.21-1.447 1.394c-.16.16-.295.295-.605.295l.213-3.053 5.56-5.023c.242-.213-.054-.333-.373-.12L7.17 13.223l-2.96-.924c-.643-.204-.657-.643.136-.953l11.57-4.461c.537-.194 1.006.131.978.336z" />
      </svg>
    ),
  },
}

type ProviderBadgeProps = {
  provider: string
}

// ProviderBadge is a compact, branded pill for a linked OAuth identity (icon + label).
export function ProviderBadge({ provider }: ProviderBadgeProps) {
  const key = provider.trim().toLowerCase()
  const meta = providerMeta[key]
  const label = meta?.label ?? provider
  const className =
    meta?.className ?? 'border-spade-cream/15 bg-spade-bg/60 text-spade-cream'

  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-spade-pill border px-2.5 py-1 text-xs font-medium tracking-wide ${className}`}
    >
      {meta?.icon ?? null}
      {label}
    </span>
  )
}
