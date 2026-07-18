import type { ReactNode } from 'react'

export const authFieldClassName =
  'rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream outline-none placeholder:text-spade-gray-3/60 focus:border-spade-gold focus:ring-2 focus:ring-spade-gold/20 disabled:cursor-not-allowed disabled:opacity-50'

export const authLabelClassName = 'grid gap-1.5 text-xs font-medium uppercase text-spade-gray-2'

export const authErrorClassName =
  'rounded-spade-md border border-spade-red/50 bg-spade-red/10 px-3 py-2 text-sm text-[#ffb4ab]'

type AuthCardShellProps = {
  title: string
  subtitle?: string
  children: ReactNode
  footer?: ReactNode
}

export function AuthCardShell({ title, subtitle, children, footer }: AuthCardShellProps) {
  return (
    <section className="grid min-h-svh place-items-center bg-spade-bg px-4 py-8">
      <div className="w-full max-w-md">
        <div className="mb-6 flex items-center justify-center gap-3">
          <img src="/logo.png" alt="Seven Spade" className="size-11" />
          <h1 className="text-2xl font-bold tracking-normal text-spade-gold-light">SEVEN SPADE</h1>
        </div>

        <div className="rounded-spade-lg border border-spade-cream/10 bg-[#102316] p-6 shadow-spade-card">
          <div className="mb-6 text-center">
            <h2 className="text-2xl font-medium leading-tight tracking-normal">{title}</h2>
            {subtitle ? <p className="mt-1.5 text-sm text-spade-gray-2">{subtitle}</p> : null}
          </div>

          {children}

          {footer ? <div className="mt-6">{footer}</div> : null}
        </div>
      </div>
    </section>
  )
}
