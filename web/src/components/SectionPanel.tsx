import type { ReactNode } from 'react'

type SectionPanelProps = {
  title: string
  eyebrow: string
  children: ReactNode
  action?: ReactNode
  // Optional surface override (bg/border/shadow). When set, replaces the default
  // dark-green table chrome so nested profile panels can match card surfaces.
  className?: string
}

export function SectionPanel({ title, eyebrow, children, action, className }: SectionPanelProps) {
  return (
    <article
      className={`rounded-spade-xl p-4 sm:p-5 ${
        className ?? 'border border-spade-green-light/25 bg-[#102316] shadow-spade-table'
      }`}
    >
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3 border-b border-spade-cream/10 pb-3">
        <div>
          <p className="font-mono text-[11px] uppercase tracking-[0.12em] text-spade-gold">{eyebrow}</p>
          <h2 className="mt-1 text-2xl font-medium tracking-normal">{title}</h2>
        </div>
        {action}
      </div>
      {children}
    </article>
  )
}
