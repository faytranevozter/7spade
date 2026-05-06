import type { ReactNode } from 'react'

type SceneShellProps = {
  title: string
  eyebrow: string
  children: ReactNode
  action?: ReactNode
}

export function SceneShell({ title, eyebrow, children, action }: SceneShellProps) {
  return (
    <section className="grid min-h-[calc(100svh-86px)] content-start gap-4 px-4 py-5 sm:px-6 lg:px-8">
      <div className="mx-auto flex w-full max-w-7xl flex-wrap items-end justify-between gap-3">
        <div>
          <p className="font-mono text-xs uppercase tracking-[0.12em] text-spade-gold">{eyebrow}</p>
          <h1 className="mt-1 text-3xl font-medium tracking-normal sm:text-[32px]">{title}</h1>
        </div>
        {action ? <div className="flex flex-wrap gap-2">{action}</div> : null}
      </div>
      <div className="mx-auto w-full max-w-7xl rounded-spade-xl border border-spade-green-light/25 bg-[#102316] p-4 shadow-spade-table sm:p-5">
        {children}
      </div>
    </section>
  )
}
